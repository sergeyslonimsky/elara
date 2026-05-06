package config_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
	mock_config "github.com/sergeyslonimsky/elara/internal/usecase/config/mocks"
)

func TestUnlockUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*config.UnlockUseCase, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.UnlockUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockunlockEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)

				store := mock_config.NewMockUnlockStore(ctrl)
				store.EXPECT().UnlockConfig(ctx, "prod", "/app/config.json").Return(nil)
				store.EXPECT().Get(ctx, "/app/config.json", "prod").Return(&domain.Config{Path: "/app/config.json", Namespace: "prod"}, nil)

				notifier := mock_config.NewMockUnlockNotifier(ctrl)
				notifier.EXPECT().NotifyConfigUnlocked(ctx, gomock.Any())

				return config.NewUnlockUseCase(enforcer, store, notifier), ctx
			},
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.UnlockUseCase, context.Context) {
				return config.NewUnlockUseCase(nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "forbidden",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.UnlockUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockunlockEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(false, nil)

				return config.NewUnlockUseCase(enforcer, nil, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "unlock error",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.UnlockUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockunlockEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)

				store := mock_config.NewMockUnlockStore(ctrl)
				store.EXPECT().UnlockConfig(ctx, "prod", "/app/config.json").Return(errors.New("db error"))

				return config.NewUnlockUseCase(enforcer, store, nil), ctx
			},
			wantErr: "unlock config: db error",
		},
		{
			name: "post-unlock read failure",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.UnlockUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockunlockEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)

				store := mock_config.NewMockUnlockStore(ctrl)
				store.EXPECT().UnlockConfig(ctx, "prod", "/app/config.json").Return(nil)
				store.EXPECT().Get(ctx, "/app/config.json", "prod").Return(nil, errors.New("read error"))

				notifier := mock_config.NewMockUnlockNotifier(ctrl)
				// Should still notify with empty payload
				notifier.EXPECT().NotifyConfigUnlocked(ctx, gomock.Any())

				return config.NewUnlockUseCase(enforcer, store, notifier), ctx
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			err := sut.Execute(ctx, "prod", "/app/config.json")

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
