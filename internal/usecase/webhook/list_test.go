package webhook_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
)

type mockLister struct {
	webhooks []*domain.Webhook
	err      error
}

func (m *mockLister) List(_ context.Context) ([]*domain.Webhook, error) {
	return m.webhooks, m.err
}

func TestListUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		webhooks []*domain.Webhook
		err      error
		wantLen  int
		wantErr  bool
	}{
		{
			name:    "empty list",
			wantLen: 0,
		},
		{
			name: "populated list",
			webhooks: []*domain.Webhook{
				{
					ID:     "wh-1",
					URL:    "https://a.example.com/hook",
					Events: []domain.WebhookEventType{domain.WebhookEventCreated},
				},
				{
					ID:     "wh-2",
					URL:    "https://b.example.com/hook",
					Events: []domain.WebhookEventType{domain.WebhookEventUpdated},
				},
			},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := webhookuc.NewListUseCase(&mockLister{webhooks: tt.webhooks, err: tt.err})
			result, err := uc.Execute(t.Context())

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, result, tt.wantLen)
			}
		})
	}
}
