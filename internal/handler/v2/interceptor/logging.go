package interceptor

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect"
)

type loggingInterceptor struct{}

// NewLoggingInterceptor returns a connect.Interceptor that logs start and
// completion (with duration and status code) for both unary and streaming RPCs.
//
//nolint:ireturn // factory must return the interface type per API contract
func NewLoggingInterceptor() connect.Interceptor {
	return &loggingInterceptor{}
}

func (i *loggingInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		start := time.Now()

		resp, err := next(ctx, req)

		slog.Info("rpc",
			"procedure", req.Spec().Procedure,
			"duration", time.Since(start),
			"code", connect.CodeOf(err).String(),
			"peer", req.Peer().Addr,
		)

		return resp, err
	}
}

func (i *loggingInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *loggingInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		start := time.Now()

		slog.Info("rpc stream start",
			"procedure", conn.Spec().Procedure,
			"peer", conn.Peer().Addr,
		)

		err := next(ctx, conn)

		slog.Info("rpc stream end",
			"procedure", conn.Spec().Procedure,
			"duration", time.Since(start),
			"code", connect.CodeOf(err).String(),
			"peer", conn.Peer().Addr,
		)

		return err
	}
}
