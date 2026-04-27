package webhook_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
)

type mockCreator struct {
	err error
}

func (m *mockCreator) Create(_ context.Context, _ *domain.Webhook) error {
	return m.err
}

func TestCreateUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		webhook *domain.Webhook
		repoErr error
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid webhook",
			webhook: &domain.Webhook{
				URL:     "https://example.com/hook",
				Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name: "invalid URL triggers validate error",
			webhook: &domain.Webhook{
				URL:    "not-a-url",
				Events: []domain.WebhookEventType{domain.WebhookEventCreated},
			},
			wantErr: true,
			errMsg:  "validate",
		},
		{
			name: "repo error propagated",
			webhook: &domain.Webhook{
				URL:     "https://example.com/hook",
				Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled: true,
			},
			repoErr: errors.New("storage failure"),
			wantErr: true,
			errMsg:  "create",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := webhookuc.NewCreateUseCase(&mockCreator{err: tt.repoErr})
			result, err := uc.Execute(t.Context(), tt.webhook)

			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}

				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}
