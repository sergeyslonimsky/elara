package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func TestService_Watch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    config.WatchInput
		mockFunc func(ctx context.Context, m mocks)
		errIs    error
	}{
		{
			name: "success",
			input: config.WatchInput{
				Namespace:  "prod",
				PathPrefix: "/app",
			},
			mockFunc: func(ctx context.Context, m mocks) {
				ch := make(chan domain.WatchEvent)
				cancel := func() {}
				m.watcher.EXPECT().Subscribe(ctx, "/app", "prod").Return(ch, cancel)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			ctx := t.Context()
			tt.mockFunc(ctx, m)

			ch, cancel, err := svc.Watch(ctx, tt.input)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			require.NoError(t, err)
			assert.NotNil(t, ch)
			assert.NotNil(t, cancel)
		})
	}
}
