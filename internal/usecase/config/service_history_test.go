package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func TestService_History(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    config.HistoryInput
		mockFunc func(ctx context.Context, m mocks) context.Context
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
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
				entries := []*domain.HistoryEntry{{Revision: 1}}
				m.storage.EXPECT().GetConfigHistory(ctx, "/a.json", "prod", 10).Return(entries, nil)

				return ctx
			},
			want: []*domain.HistoryEntry{{Revision: 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			ctx := tt.mockFunc(t.Context(), m)

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
