package interceptor_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/etcdv3/interceptor"
)

// TestTokenInterceptor_SkipPermissions covers the WithTokenSkipPermissions
// bypass path: every failure mode that normally returns Unauthenticated
// instead injects the local-admin bypass claims and lets the request through.
func TestTokenInterceptor_SkipPermissions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		buildCtx func(context.Context) context.Context
		tokens   []*domain.Token
	}{
		{
			name:     "missing authorization header",
			buildCtx: func(ctx context.Context) context.Context { return ctx },
		},
		{
			name:     "token not found",
			buildCtx: func(ctx context.Context) context.Context { return contextWithBearer(ctx, testRawToken) },
			tokens:   nil,
		},
		{
			name:     "expired token",
			buildCtx: func(ctx context.Context) context.Context { return contextWithBearer(ctx, testRawToken) },
			tokens:   []*domain.Token{expiredToken()},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := newStubTokenLookup(tc.tokens...)
			i := interceptor.NewTokenInterceptor(store, interceptor.WithTokenSkipPermissions(true))

			ctx := tc.buildCtx(t.Context())

			var capturedCtx *context.Context
			handler := func(handlerCtx context.Context, _ any) (any, error) {
				capturedCtx = &handlerCtx

				return struct{}{}, nil
			}

			_, err := i.Unary()(ctx, struct{}{}, &grpc.UnaryServerInfo{}, handler)
			require.NoError(t, err)
			require.NotNil(t, capturedCtx)

			claims, ok := authctx.ClaimsFromContext(*capturedCtx)
			require.True(t, ok)
			assert.Equal(t, "local-admin@elara.internal", claims.Email)
			assert.Equal(t, "Local Admin", claims.Name)
			assert.Equal(t, []string{"*"}, claims.Namespaces)
			assert.Equal(t, "admin", claims.Role)
		})
	}
}

// TestTokenInterceptor_ExtractPeerIP_WithPeer exercises the branch of
// extractPeerIP where peer.FromContext succeeds (grpc attached peer info to
// the incoming context), as opposed to the no-peer case exercised elsewhere.
func TestTokenInterceptor_ExtractPeerIP_WithPeer(t *testing.T) {
	t.Parallel()

	store := newStubTokenLookup(validToken())
	i := interceptor.NewTokenInterceptor(store)

	ctx := contextWithBearer(t.Context(), testRawToken)
	ctx = peer.NewContext(ctx, &peer.Peer{Addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.5"), Port: 1234}})

	handler := func(_ context.Context, _ any) (any, error) {
		return struct{}{}, nil
	}

	_, err := i.Unary()(ctx, struct{}{}, &grpc.UnaryServerInfo{}, handler)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(store.updatedHashes()) > 0
	}, time.Second, 10*time.Millisecond)
}
