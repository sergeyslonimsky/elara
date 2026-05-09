package webhook_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
	webhook_mock "github.com/sergeyslonimsky/elara/internal/usecase/webhook/mocks"
)

func TestService_Update(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		id       string
		params   webhookuc.UpdateParams
		mockFunc func(context.Context, *gomock.Controller) (*webhookuc.Service, context.Context)
		errIs    error
		wantErr  string
		want     *domain.Webhook
	}{
		{
			name: "success",
			id:   "wh-1",
			params: webhookuc.UpdateParams{
				URL:             "https://new.example.com/hook",
				NamespaceFilter: "prod",
				Events:          []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled:         true,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				enf := webhook_mock.NewMockenforcer(ctrl)
				repo := webhook_mock.NewMockrepo(ctrl)

				repo.EXPECT().Get(ctx, "wh-1").Return(&domain.Webhook{
					ID:              "wh-1",
					URL:             "https://old.example.com/hook",
					NamespaceFilter: "prod",
					Events:          []domain.WebhookEventType{domain.WebhookEventCreated},
					Enabled:         true,
				}, nil)
				enf.EXPECT().Enforce("test@example.com", "prod", auth.ObjectWebhook, auth.ActionWrite).Return(true, nil)
				repo.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				return webhookuc.New(enf, repo, nil), ctx
			},
			want: &domain.Webhook{
				ID:              "wh-1",
				URL:             "https://new.example.com/hook",
				NamespaceFilter: "prod",
				Events:          []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled:         true,
			},
		},
		{
			name: "validate error",
			id:   "wh-1",
			params: webhookuc.UpdateParams{
				URL:    "", // invalid
				Events: []domain.WebhookEventType{domain.WebhookEventCreated},
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				enf := webhook_mock.NewMockenforcer(ctrl)
				repo := webhook_mock.NewMockrepo(ctrl)

				repo.EXPECT().
					Get(ctx, "wh-1").
					Return(&domain.Webhook{ID: "wh-1", URL: "https://old.com", Events: []domain.WebhookEventType{domain.WebhookEventCreated}}, nil)
				enf.EXPECT().Enforce("test@example.com", "*", auth.ObjectWebhook, auth.ActionWrite).Return(true, nil)

				return webhookuc.New(enf, repo, nil), ctx
			},
			wantErr: "validate",
		},
		{
			name: "forbidden",
			id:   "wh-1",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				enf := webhook_mock.NewMockenforcer(ctrl)
				repo := webhook_mock.NewMockrepo(ctrl)

				repo.EXPECT().Get(ctx, "wh-1").Return(&domain.Webhook{ID: "wh-1", NamespaceFilter: "prod"}, nil)
				enf.EXPECT().
					Enforce("test@example.com", "prod", auth.ObjectWebhook, auth.ActionWrite).
					Return(false, nil)

				return webhookuc.New(enf, repo, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "repo error get",
			id:   "wh-1",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				repo := webhook_mock.NewMockrepo(ctrl)

				repo.EXPECT().Get(ctx, "wh-1").Return(nil, domain.ErrNotFound)

				return webhookuc.New(nil, repo, nil), ctx
			},
			errIs: domain.ErrNotFound,
		},
		{
			name: "repo error update",
			id:   "wh-1",
			params: webhookuc.UpdateParams{
				URL:    "https://example.com",
				Events: []domain.WebhookEventType{domain.WebhookEventCreated},
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				enf := webhook_mock.NewMockenforcer(ctrl)
				repo := webhook_mock.NewMockrepo(ctrl)

				repo.EXPECT().
					Get(ctx, "wh-1").
					Return(&domain.Webhook{ID: "wh-1", URL: "https://old.com", Events: []domain.WebhookEventType{domain.WebhookEventCreated}}, nil)
				enf.EXPECT().Enforce("test@example.com", "*", auth.ObjectWebhook, auth.ActionWrite).Return(true, nil)
				repo.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("db error"))

				return webhookuc.New(enf, repo, nil), ctx
			},
			wantErr: "db error",
		},
		{
			name: "unauthorized",
			id:   "wh-1",
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

			got, err := sut.Update(ctx, tt.id, tt.params)

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
			assert.Equal(t, tt.want.NamespaceFilter, got.NamespaceFilter)
		})
	}
}
