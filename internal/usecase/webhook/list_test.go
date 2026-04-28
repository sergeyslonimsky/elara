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
			name:    "repo error",
			err:     errors.New("db error"),
			wantErr: true,
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

			ctrl := gomock.NewController(t)
			repo := webhook_mock.NewMockwebhookLister(ctrl)
			repo.EXPECT().List(gomock.Any()).Return(tt.webhooks, tt.err)

			uc := webhookuc.NewListUseCase(repo)
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
