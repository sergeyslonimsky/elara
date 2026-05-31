package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func TestService_Update(t *testing.T) {
	t.Parallel()

	normalizedJSON := "{\n  \"key\": \"value\"\n}"

	tests := []struct {
		name     string
		input    *domain.Config
		mockFunc func(ctx context.Context, m mocks)
		errIs    error
		wantErr  string
		want     *domain.Config
	}{
		{
			name: "success with format",
			input: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   `{"key": "value"}`,
				Format:    domain.FormatJSON,
			},
			mockFunc: func(ctx context.Context, m mocks) {
				m.storage.EXPECT().
					Get(ctx, "/app/config.json", "prod").
					Return(&domain.Config{Format: domain.FormatJSON}, nil)

				m.schemaValidator.EXPECT().
					Validate(ctx, "prod", "/app/config.json", normalizedJSON, domain.FormatJSON).
					Return(nil)

				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)

				m.storage.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				m.namespaceProvider.EXPECT().UpdateTimestamp(gomock.Any(), "prod").Return(nil)
				m.watcher.EXPECT().NotifyUpdated(gomock.Any(), gomock.Any())
			},
			want: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   normalizedJSON,
				Format:    domain.FormatJSON,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			ctx := t.Context()
			tt.mockFunc(ctx, m)

			got, err := svc.Update(ctx, tt.input)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want.Path, got.Path)
			assert.Equal(t, tt.want.Namespace, got.Namespace)
			assert.Equal(t, tt.want.Content, got.Content)
			assert.Equal(t, tt.want.Format, got.Format)
		})
	}
}

func TestService_Lock(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		svc, m, _ := setupService(t)

		ctx := t.Context()
		m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			},
		)
		m.storage.EXPECT().LockConfig(gomock.Any(), "prod", "/a.json").Return(nil)

		cfg := &domain.Config{Path: "/a.json", Namespace: "prod"}
		m.storage.EXPECT().Get(gomock.Any(), "/a.json", "prod").Return(cfg, nil)
		m.watcher.EXPECT().NotifyConfigLocked(gomock.Any(), cfg)

		err := svc.Lock(ctx, config.LockInput{Namespace: "prod", Path: "/a.json"})
		require.NoError(t, err)
	})
}

func TestService_Unlock(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		svc, m, _ := setupService(t)

		ctx := t.Context()
		m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			},
		)
		m.storage.EXPECT().UnlockConfig(gomock.Any(), "prod", "/a.json").Return(nil)

		cfg := &domain.Config{Path: "/a.json", Namespace: "prod"}
		m.storage.EXPECT().Get(gomock.Any(), "/a.json", "prod").Return(cfg, nil)
		m.watcher.EXPECT().NotifyConfigUnlocked(gomock.Any(), cfg)

		err := svc.Unlock(ctx, config.UnlockInput{Namespace: "prod", Path: "/a.json"})
		require.NoError(t, err)
	})
}
