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
			name:  "all entries visible to admin",
			limit: 10,
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				entries := []*domain.ChangelogEntry{
					{Path: "/a.json", Namespace: "prod"},
					{Path: "/b.json", Namespace: "dev"},
				}
				m.activity.EXPECT().
					ListRecentChanges(ctx, 50).
					Return(entries, nil)

				m.pdp.EXPECT().
					Has("admin@example.com", domain.Permission{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "prod"}).
					Return(true)
				m.pdp.EXPECT().
					Has("admin@example.com", domain.Permission{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "dev"}).
					Return(true)

				return ctx
			},
			want: []*domain.ChangelogEntry{
				{Path: "/a.json", Namespace: "prod"},
				{Path: "/b.json", Namespace: "dev"},
			},
		},
		{
			name:  "scoped user only sees allowed-namespace entries",
			limit: 10,
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				entries := []*domain.ChangelogEntry{
					{Path: "/a.json", Namespace: "prod"},
					{Path: "/b.json", Namespace: "dev"},
					{Path: "/c.json", Namespace: "prod"},
				}
				m.activity.EXPECT().
					ListRecentChanges(ctx, 50).
					Return(entries, nil)

				// Cached per-namespace check: prod queried once, dev queried once.
				m.pdp.EXPECT().
					Has("user@example.com", domain.Permission{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "prod"}).
					Return(true)
				m.pdp.EXPECT().
					Has("user@example.com", domain.Permission{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "dev"}).
					Return(false)

				return ctx
			},
			want: []*domain.ChangelogEntry{
				{Path: "/a.json", Namespace: "prod"},
				{Path: "/c.json", Namespace: "prod"},
			},
		},
		{
			name:  "no-access user sees empty list",
			limit: 10,
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "no-access@example.com"})

				entries := []*domain.ChangelogEntry{{Path: "/a.json", Namespace: "prod"}}
				m.activity.EXPECT().
					ListRecentChanges(ctx, 50).
					Return(entries, nil)

				m.pdp.EXPECT().
					Has("no-access@example.com", domain.Permission{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "prod"}).
					Return(false)

				return ctx
			},
			want: []*domain.ChangelogEntry{},
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
			name:  "list changes error",
			limit: 10,
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				m.activity.EXPECT().
					ListRecentChanges(ctx, 50).
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
