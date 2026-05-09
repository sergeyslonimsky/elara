package namespace_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func TestService_Get(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		nsName   string
		mockFunc func(ctx context.Context, m mocks) context.Context
		errIs    error
		wantErr  string
		want     *domain.Namespace
	}{
		{
			name:   "success",
			nsName: "prod",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				m.enforcer.EXPECT().Enforce("user@example.com", "prod", "namespace", "read").Return(true, nil)
				m.store.EXPECT().Get(ctx, "prod").Return(&domain.Namespace{Name: "prod"}, nil)
				m.store.EXPECT().CountConfigs(ctx, "prod").Return(5, nil)

				return ctx
			},
			want: &domain.Namespace{Name: "prod", ConfigCount: 5},
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
				m.enforcer.EXPECT().Enforce("user@example.com", "prod", "namespace", "read").Return(false, nil)

				return ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name:   "namespace not found",
			nsName: "prod",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().Enforce("user@example.com", "prod", "namespace", "read").Return(true, nil)
				m.store.EXPECT().Get(ctx, "prod").Return(nil, domain.ErrNotFound)

				return ctx
			},
			wantErr: "get namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			got, err := svc.Get(ctx, tt.nsName)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
