package webhook_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
	webhookmock "github.com/sergeyslonimsky/elara/internal/usecase/webhook/mocks"
)

func TestService_Delete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		id       string
		mockFunc func(context.Context, *gomock.Controller) (*webhookuc.Service, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "success clears history",
			id:   "wh-1",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				pdp := webhookmock.NewMockpdp(ctrl)
				repo := webhookmock.NewMockrepo(ctrl)
				disp := webhookmock.NewMockdispatcher(ctrl)

				repo.EXPECT().
					Get(ctx, "wh-1").
					Return(&domain.Webhook{ID: "wh-1", NamespaceFilter: "prod"}, nil)
				pdp.EXPECT().
					Has("test@example.com", domain.Permission{
						Object: domain.ObjectWebhook,
						Action: domain.ActionWrite,
						Domain: "prod",
					}).
					Return(true)
				repo.EXPECT().Delete(ctx, "wh-1").Return(nil)
				disp.EXPECT().ClearHistory("wh-1")

				return webhookuc.New(pdp, repo, disp), ctx
			},
		},
		{
			name: "forbidden",
			id:   "wh-1",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				pdp := webhookmock.NewMockpdp(ctrl)
				repo := webhookmock.NewMockrepo(ctrl)

				repo.EXPECT().
					Get(ctx, "wh-1").
					Return(&domain.Webhook{ID: "wh-1", NamespaceFilter: "prod"}, nil)
				pdp.EXPECT().
					Has("test@example.com", domain.Permission{
						Object: domain.ObjectWebhook,
						Action: domain.ActionWrite,
						Domain: "prod",
					}).
					Return(false)

				return webhookuc.New(pdp, repo, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "repo error propagated",
			id:   "wh-1",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				pdp := webhookmock.NewMockpdp(ctrl)
				repo := webhookmock.NewMockrepo(ctrl)

				repo.EXPECT().Get(ctx, "wh-1").Return(&domain.Webhook{ID: "wh-1"}, nil)
				pdp.EXPECT().
					Has("test@example.com", domain.Permission{
						Object: domain.ObjectWebhook,
						Action: domain.ActionWrite,
						Domain: domain.DomainAll,
					}).
					Return(true)
				repo.EXPECT().Delete(ctx, "wh-1").Return(errors.New("db failure"))

				return webhookuc.New(pdp, repo, nil), ctx
			},
			wantErr: "db failure",
		},
		{
			name: "get error propagated",
			id:   "wh-1",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*webhookuc.Service, context.Context) {
				ctx = webhookTestCtx(ctx)
				repo := webhookmock.NewMockrepo(ctrl)

				repo.EXPECT().Get(ctx, "wh-1").Return(nil, errors.New("not found"))

				return webhookuc.New(nil, repo, nil), ctx
			},
			wantErr: "not found",
		},
		{
			name: "unauthorized",
			id:   "wh-1",
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

			err := sut.Delete(ctx, tt.id)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
		})
	}
}
