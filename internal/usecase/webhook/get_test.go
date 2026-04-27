package webhook_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
)

type mockGetter struct {
	webhook *domain.Webhook
	err     error
}

func (m *mockGetter) Get(_ context.Context, _ string) (*domain.Webhook, error) {
	return m.webhook, m.err
}

func TestGetUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		webhook *domain.Webhook
		err     error
		wantErr bool
	}{
		{
			name: "found",
			webhook: &domain.Webhook{
				ID:      "wh-1",
				URL:     "https://example.com/hook",
				Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name:    "not found",
			err:     domain.ErrNotFound,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := webhookuc.NewGetUseCase(&mockGetter{webhook: tt.webhook, err: tt.err})
			result, err := uc.Execute(t.Context(), "wh-1")

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.webhook.ID, result.ID)
			}
		})
	}
}
