package config_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func TestService_GetAtRevision(t *testing.T) {
	t.Parallel()

	expectedEntry := &domain.HistoryEntry{
		Revision: 10,
		Content:  `{"key":"val"}`,
	}

	tests := []struct {
		name     string
		input    config.GetAtRevisionInput
		mockFunc func(ctx context.Context, m mocks) context.Context
		errIs    error
		wantErr  string
		want     *domain.HistoryEntry
	}{
		{
			name: "success",
			input: config.GetAtRevisionInput{
				Namespace: "prod",
				Path:      "/db/config",
				Revision:  10,
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", "config", "read").
					Return(true, nil)
				m.storage.EXPECT().
					GetAtRevision(ctx, "/db/config", "prod", int64(10)).
					Return(expectedEntry, nil)

				return ctx
			},
			want: expectedEntry,
		},
		{
			name: "empty namespace",
			input: config.GetAtRevisionInput{
				Path:     "/db/config",
				Revision: 10,
			},
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctx
			},
			wantErr: "namespace is required",
		},
		{
			name: "missing claims",
			input: config.GetAtRevisionInput{
				Namespace: "prod",
				Path:      "/db/config",
				Revision:  10,
			},
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "enforcer error",
			input: config.GetAtRevisionInput{
				Namespace: "prod",
				Path:      "/db/config",
				Revision:  10,
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", "config", "read").
					Return(false, errors.New("db error"))

				return ctx
			},
			wantErr: "enforce: db error",
		},
		{
			name: "forbidden",
			input: config.GetAtRevisionInput{
				Namespace: "prod",
				Path:      "/db/config",
				Revision:  10,
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", "config", "read").
					Return(false, nil)

				return ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "storage error",
			input: config.GetAtRevisionInput{
				Namespace: "prod",
				Path:      "/db/config",
				Revision:  10,
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", "config", "read").
					Return(true, nil)
				m.storage.EXPECT().
					GetAtRevision(ctx, "/db/config", "prod", int64(10)).
					Return(nil, errors.New("not found"))

				return ctx
			},
			wantErr: "get config at revision: not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			ctx := tt.mockFunc(t.Context(), m)

			got, err := svc.GetAtRevision(ctx, tt.input)

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
