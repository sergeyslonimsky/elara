package interceptor_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/interceptor"
)

func TestPassthroughInterceptor_WrapUnary(t *testing.T) {
	t.Parallel()

	i := &interceptor.PassthroughInterceptor{}

	called := false
	next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		called = true
		claims, ok := auth.ClaimsFromContext(ctx)
		require.True(t, ok)
		assert.Equal(t, "local-admin@elara.internal", claims.Email)
		assert.Equal(t, "Local Admin", claims.Name)

		return connect.NewResponse(&struct{}{}), nil
	}

	_, err := i.WrapUnary(next)(t.Context(), nil)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestPassthroughInterceptor_WrapStreamingClient(t *testing.T) {
	t.Parallel()

	i := &interceptor.PassthroughInterceptor{}

	called := false
	next := func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		called = true

		return nil
	}

	res := i.WrapStreamingClient(next)(t.Context(), connect.Spec{})
	assert.Nil(t, res)
	assert.True(t, called)
}

func TestPassthroughInterceptor_WrapStreamingHandler(t *testing.T) {
	t.Parallel()

	i := &interceptor.PassthroughInterceptor{}

	called := false

	next := func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		called = true
		claims, ok := auth.ClaimsFromContext(ctx)
		require.True(t, ok)
		assert.Equal(t, "local-admin@elara.internal", claims.Email)
		assert.Equal(t, "Local Admin", claims.Name)

		return nil
	}

	err := i.WrapStreamingHandler(next)(t.Context(), nil)
	require.NoError(t, err)
	assert.True(t, called)
}
