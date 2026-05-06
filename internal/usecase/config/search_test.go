package config_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
	mock_config "github.com/sergeyslonimsky/elara/internal/usecase/config/mocks"
)

func TestSearchUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		params   config.SearchParams
		mockFunc func(context.Context, *gomock.Controller) (*config.SearchUseCase, context.Context)
		errIs    error
		wantErr  string
		want     *config.SearchResult
	}{
		{
			name: "success",
			params: config.SearchParams{
				Query:     "app",
				Namespace: "prod",
				Limit:     10,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.SearchUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMocksearchEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil).Times(2)

				configs := mock_config.NewMockconfigSearcher(ctrl)
				results := []*domain.ConfigSummary{
					{Path: "/app/1.json", Namespace: "prod"},
					{Path: "/app/2.json", Namespace: "prod"},
				}
				configs.EXPECT().SearchByPath(ctx, "app", "prod").Return(results, nil)

				return config.NewSearchUseCase(enforcer, configs), ctx
			},
			want: &config.SearchResult{
				Results: []*domain.ConfigSummary{
					{Path: "/app/1.json", Namespace: "prod"},
					{Path: "/app/2.json", Namespace: "prod"},
				},
				Total: 2,
				Limit: 10,
			},
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.SearchUseCase, context.Context) {
				return config.NewSearchUseCase(nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:   "search error",
			params: config.SearchParams{Query: "app"},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.SearchUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				configs := mock_config.NewMockconfigSearcher(ctrl)
				configs.EXPECT().SearchByPath(ctx, "app", "").Return(nil, errors.New("search error"))
				return config.NewSearchUseCase(nil, configs), ctx
			},
			wantErr: "search configs: search error",
		},
		{
			name:   "filter by permission",
			params: config.SearchParams{Query: "app"},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.SearchUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMocksearchEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
				enforcer.EXPECT().Enforce("user@example.com", "dev", "config", "read").Return(false, nil)

				configs := mock_config.NewMockconfigSearcher(ctrl)
				results := []*domain.ConfigSummary{
					{Path: "/app/1.json", Namespace: "prod"},
					{Path: "/app/2.json", Namespace: "dev"},
				}
				configs.EXPECT().SearchByPath(ctx, "app", "").Return(results, nil)

				return config.NewSearchUseCase(enforcer, configs), ctx
			},
			want: &config.SearchResult{
				Results: []*domain.ConfigSummary{
					{Path: "/app/1.json", Namespace: "prod"},
				},
				Total: 1,
				Limit: 20,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx, tt.params)

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
			assert.Equal(t, tt.want.Limit, got.Limit)
			require.Len(t, got.Results, len(tt.want.Results))
			for i := range got.Results {
				assert.Equal(t, tt.want.Results[i].Path, got.Results[i].Path)
				assert.Equal(t, tt.want.Results[i].Namespace, got.Results[i].Namespace)
			}
		})
	}
}
