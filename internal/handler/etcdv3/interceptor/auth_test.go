package interceptor_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/etcdv3/interceptor"
)

const (
	testRawToken   = "elara_testtoken123"
	testInvalidFmt = "notaprefixed_token"
)

func tokenHash(raw string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
}

// stubTokenLookup is a test double for tokenLookup.
type stubTokenLookup struct {
	mu             sync.Mutex
	tokens         map[string]*domain.Token
	updateLastUsed []string // hashes that were updated
}

func newStubTokenLookup(tokens ...*domain.Token) *stubTokenLookup {
	m := make(map[string]*domain.Token, len(tokens))
	for _, p := range tokens {
		m[p.TokenHash] = p
	}

	return &stubTokenLookup{tokens: m}
}

func (s *stubTokenLookup) GetByHash(_ context.Context, hash string) (*domain.Token, error) {
	token, ok := s.tokens[hash]
	if !ok {
		return nil, domain.NewNotFoundError("token", hash)
	}

	return token, nil
}

func (s *stubTokenLookup) UpdateLastUsed(
	_ context.Context,
	tokenHash, _ string,
	_ time.Time,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateLastUsed = append(s.updateLastUsed, tokenHash)

	return nil
}

func (s *stubTokenLookup) updatedHashes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]string, len(s.updateLastUsed))
	copy(result, s.updateLastUsed)

	return result
}

func validToken() *domain.Token {
	return &domain.Token{
		ID:         "token-1",
		IssuedBy:   "user@example.com",
		Name:       "test token",
		TokenHash:  tokenHash(testRawToken),
		Namespaces: []string{"prod"},
		Role:       "writer",
		ExpiresAt:  new(time.Now().Add(time.Hour)),
	}
}

func expiredToken() *domain.Token {
	return &domain.Token{
		ID:         "token-expired",
		IssuedBy:   "user@example.com",
		Name:       "expired token",
		TokenHash:  tokenHash(testRawToken),
		Namespaces: []string{"prod"},
		Role:       "writer",
		ExpiresAt:  new(time.Now().Add(-time.Hour)),
	}
}

func contextWithBearer(ctx context.Context, token string) context.Context {
	return metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token))
}

func TestTokenInterceptor_Unary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		buildCtx    func(context.Context) context.Context
		tokens      []*domain.Token
		wantCode    codes.Code
		wantUpdated bool
		wantClaims  bool
	}{
		{
			name:        "valid bearer token calls handler and fires update",
			buildCtx:    func(ctx context.Context) context.Context { return contextWithBearer(ctx, testRawToken) },
			tokens:      []*domain.Token{validToken()},
			wantUpdated: true,
			wantClaims:  true,
		},
		{
			name:     "missing authorization header returns unauthenticated",
			buildCtx: func(ctx context.Context) context.Context { return ctx },
			wantCode: codes.Unauthenticated,
		},
		{
			name: "no metadata in context returns unauthenticated",
			buildCtx: func(ctx context.Context) context.Context {
				return ctx // no metadata attached
			},
			wantCode: codes.Unauthenticated,
		},
		{
			name:     "token not found returns unauthenticated",
			buildCtx: func(ctx context.Context) context.Context { return contextWithBearer(ctx, testRawToken) },
			tokens:   []*domain.Token{},
			wantCode: codes.Unauthenticated,
		},
		{
			name:     "expired token returns unauthenticated",
			buildCtx: func(ctx context.Context) context.Context { return contextWithBearer(ctx, testRawToken) },
			tokens:   []*domain.Token{expiredToken()},
			wantCode: codes.Unauthenticated,
		},
		{
			name:     "token without elara_ prefix returns unauthenticated",
			buildCtx: func(ctx context.Context) context.Context { return contextWithBearer(ctx, testInvalidFmt) },
			tokens:   []*domain.Token{validToken()},
			wantCode: codes.Unauthenticated,
		},
		{
			name: "header without Bearer prefix returns unauthenticated",
			buildCtx: func(ctx context.Context) context.Context {
				return metadata.NewIncomingContext(
					ctx,
					metadata.Pairs("authorization", testRawToken),
				)
			},
			tokens:   []*domain.Token{validToken()},
			wantCode: codes.Unauthenticated,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := newStubTokenLookup(tc.tokens...)
			i := interceptor.NewTokenInterceptor(store)

			ctx := tc.buildCtx(t.Context())
			var capturedCtx *context.Context
			handler := func(handlerCtx context.Context, req any) (any, error) {
				capturedCtx = &handlerCtx

				return struct{}{}, nil
			}

			_, err := i.Unary()(ctx, struct{}{}, &grpc.UnaryServerInfo{}, handler)

			if tc.wantCode != 0 {
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tc.wantCode, st.Code())

				return
			}

			require.NoError(t, err)

			if tc.wantClaims {
				require.NotNil(t, capturedCtx)
				claims, ok := authctx.ClaimsFromContext(*capturedCtx)
				require.True(t, ok)
				assert.Equal(t, "user@example.com", claims.Email)
			}

			if tc.wantUpdated {
				// UpdateLastUsed is fire-and-forget; wait briefly for the goroutine.
				require.Eventually(t, func() bool {
					return len(store.updatedHashes()) > 0
				}, time.Second, 10*time.Millisecond)

				assert.Contains(t, store.updatedHashes(), tokenHash(testRawToken))
			}
		})
	}
}

// stubServerStream is a minimal grpc.ServerStream for testing Stream interceptor.
type stubServerStream struct {
	grpc.ServerStream

	ctx context.Context //nolint:containedctx // test helper; context stored to implement ServerStream.Context()
}

func (s *stubServerStream) Context() context.Context { return s.ctx }

func TestTokenInterceptor_Stream(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		buildCtx    func(context.Context) context.Context
		tokens      []*domain.Token
		wantCode    codes.Code
		wantUpdated bool
		wantClaims  bool
	}{
		{
			name:        "valid bearer token calls streaming handler and fires update",
			buildCtx:    func(ctx context.Context) context.Context { return contextWithBearer(ctx, testRawToken) },
			tokens:      []*domain.Token{validToken()},
			wantUpdated: true,
			wantClaims:  true,
		},
		{
			name:     "missing header on stream returns unauthenticated",
			buildCtx: func(ctx context.Context) context.Context { return ctx },
			wantCode: codes.Unauthenticated,
		},
		{
			name:     "expired token on stream returns unauthenticated",
			buildCtx: func(ctx context.Context) context.Context { return contextWithBearer(ctx, testRawToken) },
			tokens:   []*domain.Token{expiredToken()},
			wantCode: codes.Unauthenticated,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := newStubTokenLookup(tc.tokens...)
			i := interceptor.NewTokenInterceptor(store)

			ctx := tc.buildCtx(t.Context())
			ss := &stubServerStream{ctx: ctx}

			handlerCalled := false
			var capturedCtx *context.Context
			handler := func(_ any, stream grpc.ServerStream) error {
				handlerCalled = true
				capturedCtx = new(stream.Context())

				return nil
			}

			err := i.Stream()(struct{}{}, ss, &grpc.StreamServerInfo{}, handler)

			if tc.wantCode != 0 {
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tc.wantCode, st.Code())
				assert.False(t, handlerCalled)

				return
			}

			require.NoError(t, err)
			assert.True(t, handlerCalled)

			if tc.wantClaims {
				require.NotNil(t, capturedCtx)
				claims, ok := authctx.ClaimsFromContext(*capturedCtx)
				require.True(t, ok)
				assert.Equal(t, "user@example.com", claims.Email)
			}

			if tc.wantUpdated {
				require.Eventually(t, func() bool {
					return len(store.updatedHashes()) > 0
				}, time.Second, 10*time.Millisecond)

				assert.Contains(t, store.updatedHashes(), tokenHash(testRawToken))
			}
		})
	}
}
