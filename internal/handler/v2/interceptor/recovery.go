package interceptor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
)

var errInternal = errors.New("internal error")

type RecoveryInterceptor struct{}

// NewRecoveryInterceptor returns a RecoveryInterceptor that recovers from
// panics in both unary and streaming handlers.
func NewRecoveryInterceptor() *RecoveryInterceptor {
	return &RecoveryInterceptor{}
}

func (i *RecoveryInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	//nolint:nonamedreturns // resp must be named because err is named for defer-based panic recovery
	return func(ctx context.Context, req connect.AnyRequest) (resp connect.AnyResponse, err error) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic in rpc handler",
					"procedure", req.Spec().Procedure,
					"panic", r,
				)

				err = connect.NewError(connect.CodeInternal, errInternal)
			}
		}()

		return next(ctx, req)
	}
}

func (i *RecoveryInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *RecoveryInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) (err error) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic in streaming rpc handler",
					"procedure", conn.Spec().Procedure,
					"panic", r,
				)

				//nolint:err113 // wrapping runtime panic value — cannot use a sentinel
				err = connect.NewError(connect.CodeInternal, fmt.Errorf("panic: %v", r))
			}
		}()

		return next(ctx, conn)
	}
}
