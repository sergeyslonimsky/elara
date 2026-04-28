package webhook_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
	webhook_mock "github.com/sergeyslonimsky/elara/internal/usecase/webhook/mocks"
)

func TestCreateUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		webhook   *domain.Webhook
		setupMock func(repo *webhook_mock.MockwebhookCreator)
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid webhook",
			webhook: &domain.Webhook{
				URL:     "https://example.com/hook",
				Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled: true,
			},
			setupMock: func(repo *webhook_mock.MockwebhookCreator) {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "invalid URL triggers validate error",
			webhook: &domain.Webhook{
				URL:    "not-a-url",
				Events: []domain.WebhookEventType{domain.WebhookEventCreated},
			},
			setupMock: func(_ *webhook_mock.MockwebhookCreator) {
				// validation fails before Create is called
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
			setupMock: func(repo *webhook_mock.MockwebhookCreator) {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("storage failure"))
			},
			wantErr: true,
			errMsg:  "create",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repo := webhook_mock.NewMockwebhookCreator(ctrl)
			tt.setupMock(repo)

			uc := webhookuc.NewCreateUseCase(repo)
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
