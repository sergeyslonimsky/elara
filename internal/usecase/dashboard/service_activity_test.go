package dashboard_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func TestService_ListActivity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		limit    int
		mockFunc func(ctx context.Context, m mocks) context.Context
		errIs    error
		wantErr  string
		want     []*domain.ChangelogEntry
	}{
		{
			name:  "success",
			limit: 10,
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				m.enforcer.EXPECT().
					Enforce("admin@example.com", "*", "dashboard", "read").
					Return(true, nil)

				entries := []*domain.ChangelogEntry{{Path: "/a.json"}}
				m.activity.EXPECT().
					ListRecentChanges(ctx, 10).
					Return(entries, nil)

				return ctx
			},
			want: []*domain.ChangelogEntry{{Path: "/a.json"}},
		},
		{
			name:  "unauthorized",
			limit: 10,
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:  "forbidden",
			limit: 10,
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", "*", "dashboard", "read").
					Return(false, nil)

				return ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name:  "enforce error",
			limit: 10,
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().
					Enforce("admin@example.com", "*", "dashboard", "read").
					Return(false, errors.New("enforce error"))

				return ctx
			},
			wantErr: "enforce: enforce error",
		},
		{
			name:  "list changes error",
			limit: 10,
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().
					Enforce("admin@example.com", "*", "dashboard", "read").
					Return(true, nil)

				m.activity.EXPECT().
					ListRecentChanges(ctx, 10).
					Return(nil, errors.New("list error"))

				return ctx
			},
			wantErr: "list recent changes: list error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			got, err := svc.ListActivity(ctx, tt.limit)

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
