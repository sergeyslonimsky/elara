package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func TestService_Delete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    config.DeleteInput
		mockFunc func(ctx context.Context, m mocks) context.Context
		errIs    error
		wantErr  string
	}{
		{
			name: "success",
			input: config.DeleteInput{
				Namespace: "prod",
				Path:      "/app/config.json",
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).
					Return(true, nil)

				m.storage.EXPECT().Delete(ctx, "/app/config.json", "prod").Return(int64(10), nil)
				m.watcher.EXPECT().NotifyDeleted(ctx, "/app/config.json", "prod", int64(10))

				return ctx
			},
		},
		{
			name:  "unauthorized",
			input: config.DeleteInput{Namespace: "prod"},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				return ctx
			},
			errIs: domain.ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			ctx := tt.mockFunc(t.Context(), m)

			err := svc.Delete(ctx, tt.input)

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
