package interceptor

import (
	"context"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/auth"
)

// PassthroughInterceptor injects a local admin identity when auth is disabled.
type PassthroughInterceptor struct{}

func (i *PassthroughInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx = auth.WithClaims(ctx, &auth.Claims{
			Email: "local-admin@elara.internal",
			Name:  "Local Admin",
		})

		return next(ctx, req)
	}
}

func (i *PassthroughInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *PassthroughInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx = auth.WithClaims(ctx, &auth.Claims{
			Email: "local-admin@elara.internal",
			Name:  "Local Admin",
		})

		return next(ctx, conn)
	}
}
