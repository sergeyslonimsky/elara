package dashboard_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/usecase/dashboard"
)

func TestService_GetStats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(ctx context.Context, m mocks) context.Context
		errIs    error
		wantErr  string
		want     *dashboard.StatsResult
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				m.enforcer.EXPECT().
					Enforce("admin@example.com", "*", "dashboard", "read").
					Return(true, nil)

				m.namespaces.EXPECT().
					List(ctx).
					Return([]*domain.Namespace{{Name: "n1"}, {Name: "n2"}}, nil)

				m.configs.EXPECT().
					CountByNamespace(ctx, "n1").
					Return(10, nil)
				m.configs.EXPECT().
					CountByNamespace(ctx, "n2").
					Return(20, nil)
				m.configs.EXPECT().
					CurrentRevision(ctx).
					Return(int64(123), nil)

				m.activeClients.EXPECT().
					ListActive().
					Return([]*domain.Client{{ID: "c1"}})

				return ctx
			},
			want: &dashboard.StatsResult{
				NamespaceCount:    2,
				ConfigCount:       30,
				ActiveClientCount: 1,
				GlobalRevision:    123,
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
			name: "forbidden",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", "*", "dashboard", "read").
					Return(false, nil)

				return ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "count error",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				m.enforcer.EXPECT().
					Enforce("admin@example.com", "*", "dashboard", "read").
					Return(true, nil)

				m.namespaces.EXPECT().
					List(ctx).
					Return([]*domain.Namespace{{Name: "n1"}}, nil)

				m.configs.EXPECT().
					CountByNamespace(ctx, "n1").
					Return(0, errors.New("count error"))

				return ctx
			},
			wantErr: "count configs for namespace \"n1\": count error",
		},
		{
			name: "enforce error",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().
					Enforce("admin@example.com", "*", "dashboard", "read").
					Return(false, errors.New("enforce error"))

				return ctx
			},
			wantErr: "enforce: enforce error",
		},
		{
			name: "list namespaces error",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().
					Enforce("admin@example.com", "*", "dashboard", "read").
					Return(true, nil)

				m.namespaces.EXPECT().
					List(ctx).
					Return(nil, errors.New("list error"))

				return ctx
			},
			wantErr: "list namespaces: list error",
		},
		{
			name: "current revision error",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().
					Enforce("admin@example.com", "*", "dashboard", "read").
					Return(true, nil)

				m.namespaces.EXPECT().
					List(ctx).
					Return([]*domain.Namespace{}, nil)

				m.configs.EXPECT().
					CurrentRevision(ctx).
					Return(int64(0), errors.New("revision error"))

				return ctx
			},
			wantErr: "get current revision: revision error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			got, err := svc.GetStats(ctx)

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
