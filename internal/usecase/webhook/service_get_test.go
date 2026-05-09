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

func TestService_Get(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		id       string
		mockFunc func(context.Context, *gomock.Controller) (*webhookuc.Service, context.Context)
		errIs    error
		wantErr  string
		want     *domain.Webhook
	}{
		{
			name: "success",
			id:   "wh-1",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				enf := webhook_mock.NewMockenforcer(ctrl)
				repo := webhook_mock.NewMockrepo(ctrl)

				repo.EXPECT().Get(ctx, "wh-1").Return(&domain.Webhook{ID: "wh-1", NamespaceFilter: "prod"}, nil)
				enf.EXPECT().Enforce("test@example.com", "prod", auth.ObjectWebhook, auth.ActionRead).Return(true, nil)

				return webhookuc.New(enf, repo, nil), ctx
			},
			want: &domain.Webhook{ID: "wh-1", NamespaceFilter: "prod"},
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
			name: "repo error",
			id:   "wh-1",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				repo := webhook_mock.NewMockrepo(ctrl)

				repo.EXPECT().Get(ctx, "wh-1").Return(nil, errors.New("not found"))

				return webhookuc.New(nil, repo, nil), ctx
			},
			wantErr: "not found",
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

			got, err := sut.Get(ctx, tt.id)

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
