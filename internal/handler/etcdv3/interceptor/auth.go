package interceptor

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

const (
	authorizationHeader = "authorization"
	bearerPrefix        = "Bearer "
	tokenPrefix         = "elara_"
)

type tokenLookup interface {
	GetByHash(ctx context.Context, hash string) (*domain.PAT, error)
	UpdateLastUsed(ctx context.Context, tokenHash, ip string, at time.Time) error
}

// PATInterceptor validates Bearer PAT tokens from gRPC metadata.
type PATInterceptor struct {
	tokens tokenLookup
}

// NewPATInterceptor returns a PATInterceptor that authenticates requests using PATs.
func NewPATInterceptor(tokens tokenLookup) *PATInterceptor {
	return &PATInterceptor{tokens: tokens}
}

// Unary returns a gRPC unary server interceptor that validates PAT tokens.
func (i *PATInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		authedCtx, err := i.authenticate(ctx)
		if err != nil {
			return nil, err
		}

		return handler(authedCtx, req)
	}
}

// Stream returns a gRPC streaming server interceptor that validates PAT tokens.
func (i *PATInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		authedCtx, err := i.authenticate(ss.Context())
		if err != nil {
			return err
		}

		return handler(srv, &wrappedStream{ServerStream: ss, ctx: authedCtx})
	}
}

//nolint:wrapcheck // gRPC status errors are terminal; wrapping corrupts the status code
func (i *PATInterceptor) authenticate(ctx context.Context) (context.Context, error) {
	rawToken, err := extractBearerToken(ctx)
	if err != nil {
		return ctx, err
	}

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(rawToken)))

	pat, err := i.tokens.GetByHash(ctx, hash)
	if err != nil {
		return ctx, status.Error(codes.Unauthenticated, "invalid token")
	}

	if pat.IsExpired() {
		return ctx, status.Error(codes.Unauthenticated, "token expired")
	}

	peerIP := extractPeerIP(ctx)
	go func() {
		_ = i.tokens.UpdateLastUsed(context.Background(), hash, peerIP, time.Now())
	}()

	return ctx, nil
}

//nolint:wrapcheck // gRPC status errors are terminal; wrapping corrupts the status code
func extractBearerToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
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
	ctx context.Context //nolint:containedctx // standard gRPC pattern for propagating context through a wrapped stream
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}
