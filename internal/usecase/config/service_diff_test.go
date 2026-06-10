package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func TestService_Diff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    config.DiffInput
		mockFunc func(ctx context.Context, m mocks)
		wantErr  string
	}{
		{
			name: "success",
			input: config.DiffInput{
				Namespace: "prod",
				Path:      "/app.json",
				V1:        1,
				V2:        2,
			},
			mockFunc: func(ctx context.Context, m mocks) {
				m.storage.EXPECT().GetAtRevision(ctx, "/app.json", "prod", int64(2)).
					Return(&domain.HistoryEntry{Revision: 2, Content: `{"a":2}`}, nil)
				m.storage.EXPECT().GetAtRevision(ctx, "/app.json", "prod", int64(1)).
					Return(&domain.HistoryEntry{Revision: 1, Content: `{"a":1}`}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			ctx := t.Context()
			tt.mockFunc(ctx, m)

			got, err := svc.Diff(ctx, tt.input)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}
