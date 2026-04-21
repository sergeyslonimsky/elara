package interceptor

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect"
)

type LoggingInterceptor struct{}

// NewLoggingInterceptor returns a LoggingInterceptor that logs start and
// completion (with duration and status code) for both unary and streaming RPCs.
func NewLoggingInterceptor() *LoggingInterceptor {
	return &LoggingInterceptor{}
}

func (i *LoggingInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
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

func (i *LoggingInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *LoggingInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
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
