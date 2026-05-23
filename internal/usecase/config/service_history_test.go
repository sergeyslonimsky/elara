package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func TestService_History(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    config.HistoryInput
		mockFunc func(ctx context.Context, m mocks)
		errIs    error
		wantErr  string
		want     []*domain.HistoryEntry
	}{
		{
			name: "success",
			input: config.HistoryInput{
				Namespace: "prod",
				Path:      "/a.json",
				Limit:     10,
			},
			mockFunc: func(ctx context.Context, m mocks) {
				entries := []*domain.HistoryEntry{{Revision: 1}}
				m.storage.EXPECT().GetConfigHistory(ctx, "/a.json", "prod", 10).Return(entries, nil)
			},
			want: []*domain.HistoryEntry{{Revision: 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			ctx := t.Context()
			tt.mockFunc(ctx, m)

			got, err := svc.History(ctx, tt.input)

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
