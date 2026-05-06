package dashboard_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/dashboard"
	mock_dashboard "github.com/sergeyslonimsky/elara/internal/usecase/dashboard/mocks"
)

func TestUseCase_GetStats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*dashboard.UseCase, context.Context)
		errIs    error
		wantErr  string
		want     *dashboard.StatsResult
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*dashboard.UseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				enforcer := mock_dashboard.NewMockdashboardEnforcer(ctrl)
				enforcer.EXPECT().Enforce("admin@example.com", "*", "dashboard", "read").Return(true, nil)

				ns := mock_dashboard.NewMocknsLister(ctrl)
				ns.EXPECT().List(ctx).Return([]*domain.Namespace{{Name: "n1"}, {Name: "n2"}}, nil)

				configs := mock_dashboard.NewMockconfigCounter(ctrl)
				configs.EXPECT().CountByNamespace(ctx, "n1").Return(10, nil)
				configs.EXPECT().CountByNamespace(ctx, "n2").Return(20, nil)
				configs.EXPECT().CurrentRevision(ctx).Return(int64(123), nil)

				clients := mock_dashboard.NewMockactiveClientsSource(ctrl)
				clients.EXPECT().ListActive().Return([]*domain.Client{{ID: "c1"}})

				return dashboard.NewUseCase(enforcer, ns, configs, nil, clients), ctx
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
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*dashboard.UseCase, context.Context) {
				return dashboard.NewUseCase(nil, nil, nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "forbidden",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*dashboard.UseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_dashboard.NewMockdashboardEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "*", "dashboard", "read").Return(false, nil)
				return dashboard.NewUseCase(enforcer, nil, nil, nil, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "count error",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*dashboard.UseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				enforcer := mock_dashboard.NewMockdashboardEnforcer(ctrl)
				enforcer.EXPECT().Enforce("admin@example.com", "*", "dashboard", "read").Return(true, nil)

				ns := mock_dashboard.NewMocknsLister(ctrl)
				ns.EXPECT().List(ctx).Return([]*domain.Namespace{{Name: "n1"}}, nil)

				configs := mock_dashboard.NewMockconfigCounter(ctrl)
				configs.EXPECT().CountByNamespace(ctx, "n1").Return(0, errors.New("count error"))

				return dashboard.NewUseCase(enforcer, ns, configs, nil, nil), ctx
			},
			wantErr: "count configs for namespace \"n1\": count error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.GetStats(ctx)

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

func TestUseCase_ListActivity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*dashboard.UseCase, context.Context)
		errIs    error
		want     []*domain.ChangelogEntry
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*dashboard.UseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				enforcer := mock_dashboard.NewMockdashboardEnforcer(ctrl)
				enforcer.EXPECT().Enforce("admin@example.com", "*", "dashboard", "read").Return(true, nil)

				activity := mock_dashboard.NewMockactivitySource(ctrl)
				entries := []*domain.ChangelogEntry{{Path: "/a.json"}}
				activity.EXPECT().ListRecentChanges(ctx, 10).Return(entries, nil)

				return dashboard.NewUseCase(enforcer, nil, nil, activity, nil), ctx
			},
			want: []*domain.ChangelogEntry{{Path: "/a.json"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.ListActivity(ctx, 10)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
