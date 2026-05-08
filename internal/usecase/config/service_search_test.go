package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func TestService_Search(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		params   config.SearchParams
		mockFunc func(ctx context.Context, m mocks) context.Context
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
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil).Times(2)

				results := []*domain.ConfigSummary{
					{Path: "/app/1.json", Namespace: "prod"},
					{Path: "/app/2.json", Namespace: "prod"},
				}
				m.storage.EXPECT().SearchByPath(ctx, "app", "prod").Return(results, nil)

				return ctx
			},
			want: &config.SearchResult{
				Total: 2,
				Limit: 10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			ctx := tt.mockFunc(t.Context(), m)

			got, err := svc.Search(ctx, tt.params)

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
		})
	}
}
