package webhook_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
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
		{
			ID:     "wh-global",
			URL:    "https://g.example.com/hook",
			Events: []domain.WebhookEventType{domain.WebhookEventDeleted},
		},
	}

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*webhookuc.Service, context.Context)
		errIs    error
		wantErr  string
		wantIDs  []string
	}{
		{
			name: "explicit scope filters to readable namespaces",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				pdp := webhookmock.NewMockpdp(ctrl)
				repo := webhookmock.NewMockrepo(ctrl)

				repo.EXPECT().List(ctx).Return(webhooks, nil)
				pdp.EXPECT().
					EffectiveDomains("test@example.com", domain.ObjectWebhook, domain.ActionRead).
					Return(authz.NewDomainSet("prod"))

				return webhookuc.New(pdp, repo, nil), ctx
			},
			wantIDs: []string{"wh-1"},
		},
		{
			name: "wildcard scope includes global webhooks",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				pdp := webhookmock.NewMockpdp(ctrl)
				repo := webhookmock.NewMockrepo(ctrl)

				repo.EXPECT().List(ctx).Return(webhooks, nil)
				pdp.EXPECT().
					EffectiveDomains("test@example.com", domain.ObjectWebhook, domain.ActionRead).
					Return(authz.NewDomainSet("*"))

				return webhookuc.New(pdp, repo, nil), ctx
			},
			wantIDs: []string{"wh-1", "wh-2", "wh-global"},
		},
		{
			name: "empty scope returns empty without filtering",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				pdp := webhookmock.NewMockpdp(ctrl)
				repo := webhookmock.NewMockrepo(ctrl)

				repo.EXPECT().List(ctx).Return(webhooks, nil)
				pdp.EXPECT().
					EffectiveDomains("test@example.com", domain.ObjectWebhook, domain.ActionRead).
					Return(authz.NewDomainSet())

				return webhookuc.New(pdp, repo, nil), ctx
			},
			wantIDs: []string{},
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
			mockFunc: func(ctx context.Context, _ *gomock.Controller) (*webhookuc.Service, context.Context) {
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
			gotIDs := make([]string, 0, len(got))
			for _, w := range got {
				gotIDs = append(gotIDs, w.ID)
			}
			assert.ElementsMatch(t, tt.wantIDs, gotIDs)
		})
	}
}
