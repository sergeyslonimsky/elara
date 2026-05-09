package webhook_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
	webhook_mock "github.com/sergeyslonimsky/elara/internal/usecase/webhook/mocks"
)

func TestService_GetHistory(t *testing.T) {
	t.Parallel()

	now := time.Now()
	attempts := []domain.DeliveryAttempt{
		{
			AttemptNumber: 1,
			StatusCode:    200,
			LatencyMS:     42,
			Success:       true,
			Timestamp:     now,
		},
		{
			AttemptNumber: 2,
			StatusCode:    500,
			LatencyMS:     100,
			Error:         "internal server error",
			Success:       false,
			Timestamp:     now,
		},
	}

	tests := []struct {
		name     string
		id       string
		mockFunc func(context.Context, *gomock.Controller) (*webhookuc.Service, context.Context)
		errIs    error
		wantErr  string
		want     []domain.DeliveryAttempt
	}{
		{
			name: "success",
			id:   "wh-known",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				enf := webhook_mock.NewMockenforcer(ctrl)
				repo := webhook_mock.NewMockrepo(ctrl)
				disp := webhook_mock.NewMockdispatcher(ctrl)

				repo.EXPECT().Get(ctx, "wh-known").Return(&domain.Webhook{ID: "wh-known", NamespaceFilter: "prod"}, nil)
				enf.EXPECT().Enforce("test@example.com", "prod", auth.ObjectWebhook, auth.ActionRead).Return(true, nil)
				disp.EXPECT().GetDeliveryHistory("wh-known").Return(attempts)

				return webhookuc.New(enf, repo, disp), ctx
			},
			want: attempts,
		},
		{
			name: "success empty history",
			id:   "wh-empty",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				enf := webhook_mock.NewMockenforcer(ctrl)
				repo := webhook_mock.NewMockrepo(ctrl)
				disp := webhook_mock.NewMockdispatcher(ctrl)

				repo.EXPECT().Get(ctx, "wh-empty").Return(&domain.Webhook{ID: "wh-empty", NamespaceFilter: "prod"}, nil)
				enf.EXPECT().Enforce("test@example.com", "prod", auth.ObjectWebhook, auth.ActionRead).Return(true, nil)
				disp.EXPECT().GetDeliveryHistory("wh-empty").Return([]domain.DeliveryAttempt{})

				return webhookuc.New(enf, repo, disp), ctx
			},
			want: []domain.DeliveryAttempt{},
		},
		{
			name: "repo error",
			id:   "wh-unknown",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				repo := webhook_mock.NewMockrepo(ctrl)

				repo.EXPECT().Get(ctx, "wh-unknown").Return(nil, domain.ErrNotFound)

				return webhookuc.New(nil, repo, nil), ctx
			},
			errIs: domain.ErrNotFound,
		},
		{
			name: "forbidden",
			id:   "wh-1",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				enf := webhook_mock.NewMockenforcer(ctrl)
				repo := webhook_mock.NewMockrepo(ctrl)

				repo.EXPECT().Get(ctx, "wh-1").Return(&domain.Webhook{ID: "wh-1", NamespaceFilter: "prod"}, nil)
				enf.EXPECT().Enforce("test@example.com", "prod", auth.ObjectWebhook, auth.ActionRead).Return(false, nil)

				return webhookuc.New(enf, repo, nil), ctx
			},
			errIs: domain.ErrForbidden,
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

			got, err := sut.GetHistory(ctx, tt.id)

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
