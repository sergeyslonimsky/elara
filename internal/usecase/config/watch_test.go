package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
	mock_config "github.com/sergeyslonimsky/elara/internal/usecase/config/mocks"
)

func TestWatchUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		prefix   string
		ns       string
		mockFunc func(context.Context, *gomock.Controller) (*config.WatchUseCase, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name:   "success",
			prefix: "/app",
			ns:     "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.WatchUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockwatchEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)

				watch := mock_config.NewMockwatchSubscriber(ctrl)
				ch := make(chan domain.WatchEvent)
				cancel := func() {}
				watch.EXPECT().Subscribe(ctx, "/app", "prod").Return(ch, cancel)

				return config.NewWatchUseCase(enforcer, watch), ctx
			},
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.WatchUseCase, context.Context) {
				return config.NewWatchUseCase(nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "forbidden",
			ns:   "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.WatchUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_config.NewMockwatchEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(false, nil)

				return config.NewWatchUseCase(enforcer, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			ch, cancel, err := sut.Execute(ctx, tt.prefix, tt.ns)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.NotNil(t, ch)
			assert.NotNil(t, cancel)
		})
	}
}
