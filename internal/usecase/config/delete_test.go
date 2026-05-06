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

func TestDeleteUseCase_Execute(t *testing.T) {
	t.Parallel()

	type input struct {
		path      string
		namespace string
	}

	tests := []struct {
		name     string
		input    input
		mockFunc func(context.Context, *gomock.Controller) (*config.DeleteUseCase, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "success",
			input: input{
				path:      "/app/config.json",
				namespace: "prod",
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.DeleteUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockdeleteEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)

				configs := mock_config.NewMockconfigDeleter(ctrl)
				configs.EXPECT().Delete(ctx, "/app/config.json", "prod").Return(int64(10), nil)

				watch := mock_config.NewMockdeleteWatchNotifier(ctrl)
				watch.EXPECT().NotifyDeleted(ctx, "/app/config.json", "prod", int64(10))

				return config.NewDeleteUseCase(enforcer, configs, watch), ctx
			},
		},
		{
			name: "missing namespace",
			input: input{
				path: "/app/config.json",
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.DeleteUseCase, context.Context) {
				return config.NewDeleteUseCase(nil, nil, nil), ctx
			},
			wantErr: "namespace is required",
		},
		{
			name: "unauthorized",
			input: input{
				path:      "/app/config.json",
				namespace: "prod",
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.DeleteUseCase, context.Context) {
				return config.NewDeleteUseCase(nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "enforce error",
			input: input{
				path:      "/app/config.json",
				namespace: "prod",
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.DeleteUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockdeleteEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce("user@example.com", "prod", "config", "write").
					Return(false, errors.New("enforcer error"))

				return config.NewDeleteUseCase(enforcer, nil, nil), ctx
			},
			wantErr: "enforce: enforcer error",
		},
		{
			name: "forbidden",
			input: input{
				path:      "/app/config.json",
				namespace: "prod",
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.DeleteUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockdeleteEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(false, nil)

				return config.NewDeleteUseCase(enforcer, nil, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "delete config error",
			input: input{
				path:      "/app/config.json",
				namespace: "prod",
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.DeleteUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockdeleteEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)

				configs := mock_config.NewMockconfigDeleter(ctrl)
				configs.EXPECT().Delete(ctx, "/app/config.json", "prod").Return(int64(0), errors.New("db error"))

				return config.NewDeleteUseCase(enforcer, configs, nil), ctx
			},
			wantErr: "delete config: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			err := sut.Execute(ctx, tt.input.path, tt.input.namespace)

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
