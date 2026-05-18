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

func TestService_List(t *testing.T) {
	t.Parallel()

	webhooks := []*domain.Webhook{
		{
			ID:              "wh-1",
			URL:             "https://a.example.com/hook",
			Events:          []domain.WebhookEventType{domain.WebhookEventCreated},
			NamespaceFilter: "prod",
		},
		{
			ID:              "wh-2",
			URL:             "https://b.example.com/hook",
			Events:          []domain.WebhookEventType{domain.WebhookEventUpdated},
			NamespaceFilter: "dev",
		},
	}

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*webhookuc.Service, context.Context)
		errIs    error
		wantErr  string
		want     []*domain.Webhook
	}{
		{
			name: "success filters list",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				enf := webhookmock.NewMockenforcer(ctrl)
				repo := webhookmock.NewMockrepo(ctrl)

				repo.EXPECT().List(ctx).Return(webhooks, nil)
				// Allow wh-1 (prod), deny wh-2 (dev)
				enf.EXPECT().
					Enforce("test@example.com", "prod", domain.ObjectWebhook, domain.ActionRead).
					Return(true, nil)
				enf.EXPECT().
					Enforce("test@example.com", "dev", domain.ObjectWebhook, domain.ActionRead).
					Return(false, nil)

				return webhookuc.New(enf, repo, nil), ctx
			},
			want: []*domain.Webhook{webhooks[0]},
		},
		{
			name: "empty list",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				repo := webhookmock.NewMockrepo(ctrl)

				repo.EXPECT().List(ctx).Return([]*domain.Webhook{}, nil)

				return webhookuc.New(nil, repo, nil), ctx
			},
			want: []*domain.Webhook{},
		},
		{
			name: "repo error",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				repo := webhookmock.NewMockrepo(ctrl)

				repo.EXPECT().List(ctx).Return(nil, errors.New("db error"))

				return webhookuc.New(nil, repo, nil), ctx
			},
			wantErr: "db error",
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				return webhookuc.New(nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.List(ctx)

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
