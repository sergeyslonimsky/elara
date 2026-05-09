package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func TestService_Get(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    config.GetInput
		mockFunc func(ctx context.Context, m mocks) context.Context
		errIs    error
		wantErr  string
		want     *domain.Config
	}{
		{
			name: "success",
			input: config.GetInput{
				Namespace: "prod",
				Path:      "/app/config.json",
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionRead).
					Return(true, nil)

				m.storage.EXPECT().
					Get(ctx, "/app/config.json", "prod").
					Return(&domain.Config{Path: "/app/config.json", Namespace: "prod"}, nil)

				return ctx
			},
			want: &domain.Config{Path: "/app/config.json", Namespace: "prod"},
		},
		{
			name:  "unauthorized",
			input: config.GetInput{Namespace: "prod"},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				return ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:  "missing namespace",
			input: config.GetInput{Namespace: ""},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				return ctx
			},
			wantErr: "namespace is required",
		},
		{
			name:  "not found",
			input: config.GetInput{Namespace: "prod", Path: "/missing"},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionRead).
					Return(true, nil)

				m.storage.EXPECT().
					Get(ctx, "/missing", "prod").
					Return(nil, domain.ErrNotFound)

				return ctx
			},
			errIs: domain.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			ctx := tt.mockFunc(t.Context(), m)

			got, err := svc.Get(ctx, tt.input)

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
		})
	}
}
