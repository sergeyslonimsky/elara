package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestService_Update(t *testing.T) {
	t.Parallel()

	normalizedJSON := "{\n  \"key\": \"value\"\n}"

	tests := []struct {
		name     string
		input    *domain.Config
		mockFunc func(ctx context.Context, m mocks) context.Context
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
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).
					Return(true, nil)

				m.storage.EXPECT().
					Get(ctx, "/app/config.json", "prod").
					Return(&domain.Config{Format: domain.FormatJSON}, nil)

				m.schemaValidator.EXPECT().
					Validate(ctx, "prod", "/app/config.json", normalizedJSON, domain.FormatJSON).
					Return(nil)

				m.storage.EXPECT().Update(ctx, gomock.Any()).Return(nil)
				m.namespaceProvider.EXPECT().UpdateTimestamp(ctx, "prod").Return(nil)
				m.watcher.EXPECT().NotifyUpdated(ctx, gomock.Any())

				return ctx
			},
			want: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   normalizedJSON,
				Format:    domain.FormatJSON,
			},
		},
		{
			name:  "unauthorized",
			input: &domain.Config{Namespace: "prod"},
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
