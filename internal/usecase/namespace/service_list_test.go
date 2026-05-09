package namespace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/usecase/namespace"
)

func TestService_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		params   namespace.ListParams
		mockFunc func(ctx context.Context, m mocks) context.Context
		errIs    error
		wantErr  string
		want     *namespace.ListResult
	}{
		{
			name: "success",
			params: namespace.ListParams{
				Limit: 10,
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				m.store.EXPECT().List(ctx).Return([]*domain.Namespace{
					{Name: "prod"},
					{Name: "dev"},
				}, nil)

				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectNamespace, auth.ActionRead).
					Return(true, nil)
				m.enforcer.EXPECT().
					Enforce("user@example.com", "dev", auth.ObjectNamespace, auth.ActionRead).
					Return(true, nil)

				m.store.EXPECT().CountConfigs(ctx, "dev").Return(5, nil)
				m.store.EXPECT().CountConfigs(ctx, "prod").Return(10, nil)

				return ctx
			},
			want: &namespace.ListResult{
				Namespaces: []*domain.Namespace{
					{Name: "dev", ConfigCount: 5},
					{Name: "prod", ConfigCount: 10},
				},
				Total: 2,
				Limit: 10,
			},
		},
		{
			name:   "filter by permission",
			params: namespace.ListParams{Limit: 10},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				m.store.EXPECT().List(ctx).Return([]*domain.Namespace{
					{Name: "prod"},
					{Name: "dev"},
				}, nil)

				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectNamespace, auth.ActionRead).
					Return(true, nil)
				m.enforcer.EXPECT().
					Enforce("user@example.com", "dev", auth.ObjectNamespace, auth.ActionRead).
					Return(false, nil)

				m.store.EXPECT().CountConfigs(ctx, "prod").Return(10, nil)

				return ctx
			},
			want: &namespace.ListResult{
				Namespaces: []*domain.Namespace{
					{Name: "prod", ConfigCount: 10},
				},
				Total: 1,
				Limit: 10,
			},
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "list error",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.store.EXPECT().List(ctx).Return(nil, errors.New("db error"))

				return ctx
			},
			wantErr: "list namespaces: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			got, err := svc.List(ctx, tt.params)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want.Total, got.Total)
			require.Len(t, got.Namespaces, len(tt.want.Namespaces))
			for i := range got.Namespaces {
				assert.Equal(t, tt.want.Namespaces[i].Name, got.Namespaces[i].Name)
				assert.Equal(t, tt.want.Namespaces[i].ConfigCount, got.Namespaces[i].ConfigCount)
			}
		})
	}
}
