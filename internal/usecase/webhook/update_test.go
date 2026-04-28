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
				URL:     "https://example.com/hook",
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
				URL:             "https://example.com/hook",
				NamespaceFilter: "production",
				Events:          []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled:         true,
			},
			wantURL:     "https://example.com/hook",
			wantEnabled: true,
			wantNSF:     "production",
			wantEvents:  []domain.WebhookEventType{domain.WebhookEventCreated},
		},
		{
			name: "empty URL triggers validation error",
			existing: &domain.Webhook{
				ID:      "wh-4",
				URL:     "https://example.com/hook",
				Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled: true,
			},
			params: webhookuc.UpdateParams{
				URL:     "",
				Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled: true,
			},
			wantErr: true,
		},
		{
			name: "empty events list triggers validation error",
			existing: &domain.Webhook{
				ID:      "wh-5",
				URL:     "https://example.com/hook",
				Events:  []domain.WebhookEventType{domain.WebhookEventDeleted},
				Enabled: true,
			},
			params: webhookuc.UpdateParams{
				URL:     "https://example.com/hook",
				Events:  nil,
				Enabled: true,
			},
			wantErr: true,
		},
		{
			name: "secret updated when non-empty",
			existing: &domain.Webhook{
				ID:      "wh-6",
				URL:     "https://example.com/hook",
				Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
				Secret:  "old-secret",
				Enabled: true,
			},
			params: webhookuc.UpdateParams{
				URL:     "https://example.com/hook",
				Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
				Secret:  "new-secret",
				Enabled: true,
			},
			wantURL:     "https://example.com/hook",
			wantEnabled: true,
			wantEvents:  []domain.WebhookEventType{domain.WebhookEventCreated},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repo := webhook_mock.NewMockwebhookUpdater(ctrl)

			repo.EXPECT().Get(gomock.Any(), tt.existing.ID).Return(tt.existing, nil)

			if !tt.wantErr {
				repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
			}

			uc := webhookuc.NewUpdateUseCase(repo)
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

func TestUpdateUseCase_Execute_ValidationError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := webhook_mock.NewMockwebhookUpdater(ctrl)

	repo.EXPECT().Get(gomock.Any(), "wh-1").Return(&domain.Webhook{
		ID:     "wh-1",
		URL:    "https://example.com/hook",
		Events: []domain.WebhookEventType{domain.WebhookEventCreated},
	}, nil)

	uc := webhookuc.NewUpdateUseCase(repo)

	// Setting URL to an invalid value triggers a validation error.
	_, err := uc.Execute(t.Context(), "wh-1", webhookuc.UpdateParams{
		URL:    "not-a-valid-url",
		Events: []domain.WebhookEventType{domain.WebhookEventCreated},
	})

	require.Error(t, err)
	assert.True(t, domain.IsValidationError(err))
}

func TestUpdateUseCase_Execute_GetError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := webhook_mock.NewMockwebhookUpdater(ctrl)

	repo.EXPECT().Get(gomock.Any(), "missing").Return(nil, domain.ErrNotFound)

	uc := webhookuc.NewUpdateUseCase(repo)

	_, err := uc.Execute(t.Context(), "missing", webhookuc.UpdateParams{
		URL:    "https://example.com/hook",
		Events: []domain.WebhookEventType{domain.WebhookEventCreated},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestUpdateUseCase_Execute_UpdateError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := webhook_mock.NewMockwebhookUpdater(ctrl)

	repo.EXPECT().Get(gomock.Any(), "wh-1").Return(&domain.Webhook{
		ID:     "wh-1",
		URL:    "https://example.com/hook",
		Events: []domain.WebhookEventType{domain.WebhookEventCreated},
	}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(domain.ErrNotFound)

	uc := webhookuc.NewUpdateUseCase(repo)

	_, err := uc.Execute(t.Context(), "wh-1", webhookuc.UpdateParams{
		URL:    "https://example.com/hook",
		Events: []domain.WebhookEventType{domain.WebhookEventCreated},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
