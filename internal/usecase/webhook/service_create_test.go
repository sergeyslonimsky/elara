package webhook_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
	webhookmock "github.com/sergeyslonimsky/elara/internal/usecase/webhook/mocks"
)

func TestService_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		webhook  *domain.Webhook
		mockFunc func(context.Context, *gomock.Controller) (*webhookuc.Service, context.Context)
		errIs    error
		wantErr  string
		want     *domain.Webhook
	}{
		{
			name: "success",
			webhook: &domain.Webhook{
				URL:             "https://example.com/hook",
				NamespaceFilter: "prod",
				Events:          []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled:         true,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				repo := webhookmock.NewMockrepo(ctrl)
				repo.EXPECT().Create(ctx, gomock.Any()).Return(nil)

				return webhookuc.New(nil, repo, nil), ctx
			},
			want: &domain.Webhook{
				URL:             "https://example.com/hook",
				NamespaceFilter: "prod",
				Events:          []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled:         true,
			},
		},
		{
			name: "validate error",
			webhook: &domain.Webhook{
				URL:    "not-a-url",
				Events: []domain.WebhookEventType{domain.WebhookEventCreated},
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				return webhookuc.New(nil, nil, nil), ctx
			},
			wantErr: "validate",
		},
		{
			name: "repo error",
			webhook: &domain.Webhook{
				URL:     "https://example.com/hook",
				Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled: true,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				repo := webhookmock.NewMockrepo(ctrl)
				repo.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db error"))

				return webhookuc.New(nil, repo, nil), ctx
			},
			wantErr: "create",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Create(ctx, tt.webhook)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want.URL, got.URL)
		})
	}
}
