package interceptor

import (
	"context"
	"strings"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	auth2 "github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	tokenuc "github.com/sergeyslonimsky/elara/internal/usecase/token"
)

const (
	authorizationHeader = "authorization"
	bearerPrefix        = "Bearer "
	tokenPrefix         = "elara_"
)

type tokenLookup interface {
	GetByHash(ctx context.Context, hash string) (*domain.Token, error)
	UpdateLastUsed(ctx context.Context, tokenHash, ip string, at time.Time) error
}

// TokenInterceptor validates service tokens from gRPC metadata. Two carriers
// are accepted, both holding the raw token:
//   - `token: <raw>` — what clientv3 and etcdctl send after Auth.Authenticate
//     (or with clientv3.Config.Token);
//   - `authorization: Bearer <raw>` — the original Elara form.
type TokenInterceptor struct {
	tokens          tokenLookup
	skipPermissions bool
}

type TokenInterceptorOption func(*TokenInterceptor)

func WithTokenSkipPermissions(skip bool) TokenInterceptorOption {
	return func(i *TokenInterceptor) {
		i.skipPermissions = skip
	}
}

// NewTokenInterceptor returns a TokenInterceptor that authenticates requests using service tokens.
func NewTokenInterceptor(tokens tokenLookup, opts ...TokenInterceptorOption) *TokenInterceptor {
	i := &TokenInterceptor{tokens: tokens}
	for _, opt := range opts {
		opt(i)
	}

	return i
}

// Unary returns a gRPC unary server interceptor that validates service tokens.
func (i *TokenInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// Authenticate is how a client obtains credentials; it can't require them.
		if info.FullMethod == etcdserverpb.Auth_Authenticate_FullMethodName {
			return handler(ctx, req)
		}

		authedCtx, err := i.authenticate(ctx)
		if err != nil {
			return nil, err
		}

		return handler(authedCtx, req)
	}
}

// Stream returns a gRPC streaming server interceptor that validates service tokens.
func (i *TokenInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		authedCtx, err := i.authenticate(ss.Context())
		if err != nil {
			return err
		}

		return handler(srv, &wrappedStream{ServerStream: ss, ctx: authedCtx})
	}
}

func (i *TokenInterceptor) injectBypassClaims(ctx context.Context) context.Context {
	claims := &auth2.Claims{
		Email:      "local-admin@elara.internal",
		Name:       "Local Admin",
		Namespaces: []string{"*"},
		Role:       "admin",
	}

	return auth2.WithClaims(ctx, claims)
}

// authenticate extracts and validates the bearer token, injecting claims into the context.
func (i *TokenInterceptor) authenticate(ctx context.Context) (context.Context, error) {
	rawToken, err := extractRawToken(ctx)
	if err != nil {
		if i.skipPermissions {
			return i.injectBypassClaims(ctx), nil
		}

		return ctx, err
	}

	hash := tokenuc.HashRawToken(rawToken)

	token, err := i.tokens.GetByHash(ctx, hash)
	if err != nil {
		if i.skipPermissions {
			return i.injectBypassClaims(ctx), nil
		}

		// The etcd-specific message is what makes clientv3 re-run
		// Authenticate; for Bearer callers the code is still Unauthenticated.
		return ctx, rpctypes.ErrGRPCInvalidAuthToken
	}

	if token.IsExpired() {
		if i.skipPermissions {
			return i.injectBypassClaims(ctx), nil
		}

		return ctx, rpctypes.ErrGRPCInvalidAuthToken
	}

	peerIP := extractPeerIP(ctx)
	go func() {
		_ = i.tokens.UpdateLastUsed(context.Background(), hash, peerIP, time.Now())
	}()

	// Inject claims so usecases/handlers can check namespace/role scope.
	claims := &auth2.Claims{
		Email:      token.IssuedBy,
		Name:       token.Name,
		Namespaces: token.Namespaces,
		Role:       string(token.Role),
	}

	return auth2.WithClaims(ctx, claims), nil
}

//nolint:wrapcheck // gRPC status errors are terminal; wrapping corrupts the status code
func extractRawToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}

	if values := md.Get(rpctypes.TokenFieldNameGRPC); len(values) > 0 && values[0] != "" {
		if !strings.HasPrefix(values[0], tokenPrefix) {
			return "", rpctypes.ErrGRPCInvalidAuthToken
		}

		return values[0], nil
	}

	values := md.Get(authorizationHeader)
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}

	authHeader := values[0]
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return "", status.Error(codes.Unauthenticated, "invalid authorization header format")
	}

	rawToken := strings.TrimPrefix(authHeader, bearerPrefix)
	if !strings.HasPrefix(rawToken, tokenPrefix) {
		return "", status.Error(codes.Unauthenticated, "invalid token format")
	}

	return rawToken, nil
}

func extractPeerIP(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return ""
	}

	return p.Addr.String()
}

// wrappedStream replaces the context of a gRPC ServerStream.
type wrappedStream struct {
	grpc.ServerStream

	//nolint:containedctx //NOSONAR standard gRPC pattern: context stored to override ServerStream.Context()
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}
