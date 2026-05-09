package namespace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func TestService_Delete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		nsName   string
		mockFunc func(ctx context.Context, m mocks) context.Context
		errIs    error
		wantErr  string
	}{
		{
			name:   "success",
			nsName: "prod",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				m.enforcer.EXPECT().Enforce("admin@example.com", "prod", "namespace", "write").Return(true, nil)
				m.store.EXPECT().CountConfigs(ctx, "prod").Return(0, nil)
				m.store.EXPECT().Delete(ctx, "prod").Return(nil)

				return ctx
			},
		},
		{
			name:   "unauthorized",
			nsName: "prod",
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:   "forbidden",
			nsName: "prod",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().Enforce("user@example.com", "prod", "namespace", "write").Return(false, nil)

				return ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name:   "not empty",
			nsName: "prod",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				m.enforcer.EXPECT().Enforce("admin@example.com", "prod", "namespace", "write").Return(true, nil)
				m.store.EXPECT().CountConfigs(ctx, "prod").Return(5, nil)

				return ctx
			},
			wantErr: `namespace "prod" contains 5 config(s)`,
		},
		{
			name:   "delete error",
			nsName: "prod",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				m.enforcer.EXPECT().Enforce("admin@example.com", "prod", "namespace", "write").Return(true, nil)
				m.store.EXPECT().CountConfigs(ctx, "prod").Return(0, nil)
				m.store.EXPECT().Delete(ctx, "prod").Return(errors.New("db error"))

				return ctx
			},
			wantErr: "delete namespace: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			err := svc.Delete(ctx, tt.nsName)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}
