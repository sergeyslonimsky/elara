package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func TestService_Delete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    config.DeleteInput
		mockFunc func(ctx context.Context, m mocks)
		errIs    error
		wantErr  string
	}{
		{
			name: "success",
			input: config.DeleteInput{
				Namespace: "prod",
				Path:      "/app/config.json",
			},
			mockFunc: func(ctx context.Context, m mocks) {
				m.storage.EXPECT().Delete(ctx, "/app/config.json", "prod").Return(int64(10), nil)
				m.watcher.EXPECT().NotifyDeleted(ctx, "/app/config.json", "prod", int64(10))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			ctx := t.Context()
			tt.mockFunc(ctx, m)

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
