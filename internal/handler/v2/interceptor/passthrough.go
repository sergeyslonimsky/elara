package interceptor

import (
	"context"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

// PassthroughInterceptor injects a synthetic local-admin identity into the
// request context when UI auth is disabled. Pair it with NewRBACInterceptor so
// the same enforcement code path runs in both modes — the passthrough user is
// seeded as a member of the system admins group on startup, so RBAC will
// authorize anything it asks for.
type PassthroughInterceptor struct{}

var _ connect.Interceptor = (*PassthroughInterceptor)(nil)

func (i *PassthroughInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		return next(passthroughCtx(ctx), req)
	}
}

func (i *PassthroughInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *PassthroughInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		return next(passthroughCtx(ctx), conn)
	}
}

func passthroughCtx(ctx context.Context) context.Context {
	return auth.WithClaims(ctx, &auth.Claims{
		Email: auth.PassthroughEmail,
		Name:  "Local Admin",
	})
}
