package config_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
	mock_config "github.com/sergeyslonimsky/elara/internal/usecase/config/mocks"
)

func TestGetUseCase_Execute(t *testing.T) {
	t.Parallel()

	type input struct {
		path      string
		namespace string
	}

	tests := []struct {
		name     string
		input    input
		mockFunc func(context.Context, *gomock.Controller) (*config.GetUseCase, context.Context)
		errIs    error
		wantErr  string
		want     *domain.Config
	}{
		{
			name: "success",
			input: input{
				path:      "/app/config.json",
				namespace: "prod",
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.GetUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockgetEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)

				configs := mock_config.NewMockconfigGetter(ctrl)
				expectedCfg := &domain.Config{Path: "/app/config.json"}
				configs.EXPECT().Get(ctx, "/app/config.json", "prod").Return(expectedCfg, nil)

				return config.NewGetUseCase(enforcer, configs), ctx
			},
			want: &domain.Config{Path: "/app/config.json"},
		},
		{
			name: "missing namespace",
			input: input{
				path: "/app/config.json",
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.GetUseCase, context.Context) {
				return config.NewGetUseCase(nil, nil), ctx
			},
			wantErr: "namespace is required",
		},
		{
			name: "unauthorized",
			input: input{
				path:      "/app/config.json",
				namespace: "prod",
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.GetUseCase, context.Context) {
				return config.NewGetUseCase(nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "enforce error",
			input: input{
				path:      "/app/config.json",
				namespace: "prod",
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.GetUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockgetEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(false, errors.New("enforcer error"))

				return config.NewGetUseCase(enforcer, nil), ctx
			},
			wantErr: "enforce: enforcer error",
		},
		{
			name: "forbidden",
			input: input{
				path:      "/app/config.json",
				namespace: "prod",
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.GetUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockgetEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(false, nil)

				return config.NewGetUseCase(enforcer, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "get config error",
			input: input{
				path:      "/app/config.json",
				namespace: "prod",
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.GetUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockgetEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)

				configs := mock_config.NewMockconfigGetter(ctrl)
				configs.EXPECT().Get(ctx, "/app/config.json", "prod").Return(nil, errors.New("db error"))

				return config.NewGetUseCase(enforcer, configs), ctx
			},
			wantErr: "get config: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx, tt.input.path, tt.input.namespace)

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
