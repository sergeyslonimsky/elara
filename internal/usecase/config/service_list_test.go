package config_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func TestService_List(t *testing.T) {
	t.Parallel()

	now := time.Now()

	summaries := []*domain.ConfigSummary{
		{Path: "/services/api/config.json", Format: domain.FormatJSON, Version: 1, UpdatedAt: now},
		{Path: "/services/api/secrets.yaml", Format: domain.FormatYAML, Version: 2, UpdatedAt: now.Add(-time.Hour)},
		{
			Path:      "/services/web/config.json",
			Format:    domain.FormatJSON,
			Version:   1,
			UpdatedAt: now.Add(-2 * time.Hour),
		},
		{Path: "/databases/pg.json", Format: domain.FormatJSON, Version: 3, UpdatedAt: now},
		{Path: "/config.json", Format: domain.FormatJSON, Version: 1, UpdatedAt: now},
	}

	tests := []struct {
		name     string
		params   config.ListParams
		mockFunc func(ctx context.Context, m mocks)
		want     *config.ListResult
	}{
		{
			name: "root level browsing",
			params: config.ListParams{
				Namespace: "default",
				Path:      "/",
				Limit:     50,
			},
			mockFunc: func(ctx context.Context, m mocks) {
				m.storage.EXPECT().ListSummariesByPrefix(ctx, "/", "default").Return(summaries, nil)
			},
			want: &config.ListResult{
				Total: 3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			ctx := t.Context()
			tt.mockFunc(ctx, m)

			result, err := svc.List(ctx, tt.params)

			require.NoError(t, err)
			assert.Equal(t, tt.want.Total, result.Total)
			if tt.name == "root level browsing" {
				require.Len(t, result.Entries, 3)
				assert.Equal(t, "databases", result.Entries[0].Name)
				assert.Equal(t, "services", result.Entries[1].Name)
				assert.Equal(t, "config.json", result.Entries[2].Name)
			}
		})
	}
}
