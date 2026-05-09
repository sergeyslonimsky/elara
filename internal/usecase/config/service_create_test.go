package config_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func TestService_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *domain.Config
		mockFunc func(ctx context.Context, m mocks) context.Context
		errIs    error
		wantErr  string
		want     *domain.Config
	}{
		{
			name: "success",
			input: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   `{"key": "value"}`,
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).
					Return(true, nil)

				m.namespaceProvider.EXPECT().Get(ctx, "prod").Return(&domain.Namespace{Name: "prod"}, nil)

				normalized := "{\n  \"key\": \"value\"\n}"
				m.schemaValidator.EXPECT().
					Validate(ctx, "prod", "/app/config.json", normalized, domain.FormatJSON).
					Return(nil)

				m.storage.EXPECT().Create(ctx, gomock.Any()).Return(nil)
				m.namespaceProvider.EXPECT().UpdateTimestamp(ctx, "prod").Return(nil)
				m.watcher.EXPECT().NotifyCreated(ctx, gomock.Any())

				return ctx
			},
			want: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   "{\n  \"key\": \"value\"\n}",
				Format:    domain.FormatJSON,
				Version:   1,
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
		{
			name:  "invalid path",
			input: &domain.Config{Path: "invalid", Namespace: "prod"},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).
					Return(true, nil)

				return ctx
			},
			wantErr: "validate path",
		},
		{
			name:  "namespace does not exist",
			input: &domain.Config{Path: "/app/config.json", Namespace: "prod"},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).
					Return(true, nil)

				m.namespaceProvider.EXPECT().Get(ctx, "prod").Return(nil, domain.ErrNotFound)

				return ctx
			},
			wantErr: `namespace "prod" does not exist`,
		},
		{
			name: "invalid content",
			input: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   `{invalid json}`,
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).
					Return(true, nil)

				m.namespaceProvider.EXPECT().Get(ctx, "prod").Return(&domain.Namespace{Name: "prod"}, nil)

				return ctx
			},
			wantErr: "validate content",
		},
		{
			name: "schema validation error",
			input: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   `{"key": "value"}`,
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).
					Return(true, nil)

				m.namespaceProvider.EXPECT().Get(ctx, "prod").Return(&domain.Namespace{Name: "prod"}, nil)

				normalized := "{\n  \"key\": \"value\"\n}"
				m.schemaValidator.EXPECT().
					Validate(ctx, "prod", "/app/config.json", normalized, domain.FormatJSON).
					Return(errors.New("schema error"))

				return ctx
			},
			wantErr: "schema validation: schema error",
		},
		{
			name: "create config error",
			input: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   `{"key": "value"}`,
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).
					Return(true, nil)

				m.namespaceProvider.EXPECT().Get(ctx, "prod").Return(&domain.Namespace{Name: "prod"}, nil)

				normalized := "{\n  \"key\": \"value\"\n}"
				m.schemaValidator.EXPECT().
					Validate(ctx, "prod", "/app/config.json", normalized, domain.FormatJSON).
					Return(nil)

				m.storage.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db error"))

				return ctx
			},
			wantErr: "create config: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			ctx := tt.mockFunc(t.Context(), m)

			got, err := svc.Create(ctx, tt.input)

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
			assert.Equal(t, tt.want.Version, got.Version)
		})
	}
}
