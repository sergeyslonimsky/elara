package webhook_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
)

type mockUpdater struct {
	webhook *domain.Webhook
	getErr  error
	upErr   error
}

func (m *mockUpdater) Get(_ context.Context, _ string) (*domain.Webhook, error) {
	return m.webhook, m.getErr
}

func (m *mockUpdater) Update(_ context.Context, w *domain.Webhook) error {
	if m.upErr != nil {
		return m.upErr
	}

	m.webhook = w

	return nil
}

func TestUpdateUseCase_Execute_MergesCorrectly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		existing    *domain.Webhook
		params      webhookuc.UpdateParams
		wantURL     string
		wantEnabled bool
		wantEvents  []domain.WebhookEventType
		wantNSF     string
		wantErr     bool
	}{
		{
			name: "partial update - URL changed",
			existing: &domain.Webhook{
				ID:      "wh-1",
				URL:     "https://old.example.com/hook",
				Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled: true,
			},
			params: webhookuc.UpdateParams{
				URL:     "https://new.example.com/hook",
				Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled: true,
			},
			wantURL:     "https://new.example.com/hook",
			wantEnabled: true,
			wantEvents:  []domain.WebhookEventType{domain.WebhookEventCreated},
		},
		{
			name: "update events and disable",
			existing: &domain.Webhook{
				ID:      "wh-2",
				URL:     "https://example.com/hook",
				Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled: true,
			},
			params: webhookuc.UpdateParams{
				Events:  []domain.WebhookEventType{domain.WebhookEventUpdated, domain.WebhookEventDeleted},
				Enabled: false,
			},
			wantURL:     "https://example.com/hook",
			wantEnabled: false,
			wantEvents:  []domain.WebhookEventType{domain.WebhookEventUpdated, domain.WebhookEventDeleted},
		},
		{
			name: "namespace filter applied",
			existing: &domain.Webhook{
				ID:      "wh-3",
				URL:     "https://example.com/hook",
				Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled: true,
			},
			params: webhookuc.UpdateParams{
				NamespaceFilter: "production",
				Events:          []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled:         true,
			},
			wantURL:     "https://example.com/hook",
			wantEnabled: true,
			wantNSF:     "production",
			wantEvents:  []domain.WebhookEventType{domain.WebhookEventCreated},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &mockUpdater{webhook: tt.existing}
			uc := webhookuc.NewUpdateUseCase(m)

			result, err := uc.Execute(t.Context(), tt.existing.ID, tt.params)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantURL, result.URL)
				assert.Equal(t, tt.wantEnabled, result.Enabled)
				assert.Equal(t, tt.wantEvents, result.Events)
				assert.Equal(t, tt.wantNSF, result.NamespaceFilter)
			}
		})
	}
}
