package webhook_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
	webhook_mock "github.com/sergeyslonimsky/elara/internal/usecase/webhook/mocks"
)

func TestGetUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupMock func(repo *webhook_mock.MockwebhookGetter)
		wantErr   bool
		wantID    string
	}{
		{
			name: "found",
			setupMock: func(repo *webhook_mock.MockwebhookGetter) {
				repo.EXPECT().Get(gomock.Any(), "wh-1").Return(&domain.Webhook{
					ID:      "wh-1",
					URL:     "https://example.com/hook",
					Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
					Enabled: true,
				}, nil)
			},
			wantErr: false,
			wantID:  "wh-1",
		},
		{
			name: "not found",
			setupMock: func(repo *webhook_mock.MockwebhookGetter) {
				repo.EXPECT().Get(gomock.Any(), "wh-1").Return(nil, domain.ErrNotFound)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repo := webhook_mock.NewMockwebhookGetter(ctrl)
			tt.setupMock(repo)

			uc := webhookuc.NewGetUseCase(repo)
			result, err := uc.Execute(t.Context(), "wh-1")

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantID, result.ID)
			}
		})
	}
}
