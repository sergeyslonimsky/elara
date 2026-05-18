package namespace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/usecase/namespace"
)

// List is authenticated-only at the handler boundary; the usecase uses the
// caller's claims for business-logic filtering and per-namespace permission
// flags (CanRead, CanWrite).

func TestService_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		params   namespace.ListParams
		mockFunc func(ctx context.Context, m mocks) context.Context
		errIs    error
		wantErr  string
		wantLen  int
		wantWant map[string]struct{ canWrite bool }
	}{
		{
			name:   "success — annotates each visible namespace with CanRead/CanWrite",
			params: namespace.ListParams{Limit: 10},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				m.store.EXPECT().List(ctx).Return([]*domain.Namespace{
					{Name: "prod"},
					{Name: "dev"},
				}, nil)

				// filter pass: both visible
				m.pdp.EXPECT().
					Has("user@example.com", domain.Permission{
						Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "prod",
					}).
					Return(true)
				m.pdp.EXPECT().
					Has("user@example.com", domain.Permission{
						Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "dev",
					}).
					Return(true)

				m.store.EXPECT().CountConfigs(ctx, "dev").Return(5, nil)
				m.store.EXPECT().CountConfigs(ctx, "prod").Return(10, nil)

				// per-namespace CanWrite flag computation
				m.pdp.EXPECT().
					Has("user@example.com", domain.Permission{
						Object: domain.ObjectConfig, Action: domain.ActionWrite, Domain: "dev",
					}).
					Return(true)
				m.pdp.EXPECT().
					Has("user@example.com", domain.Permission{
						Object: domain.ObjectConfig, Action: domain.ActionWrite, Domain: "prod",
					}).
					Return(false)

				return ctx
			},
			wantLen: 2,
			wantWant: map[string]struct{ canWrite bool }{
				"dev":  {canWrite: true},
				"prod": {canWrite: false},
			},
		},
		{
			name:   "filter by namespace/read drops invisible items",
			params: namespace.ListParams{Limit: 10},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				m.store.EXPECT().List(ctx).Return([]*domain.Namespace{
					{Name: "prod"},
					{Name: "dev"},
				}, nil)

				m.pdp.EXPECT().
					Has("user@example.com", domain.Permission{
						Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "prod",
					}).
					Return(true)
				m.pdp.EXPECT().
					Has("user@example.com", domain.Permission{
						Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "dev",
					}).
					Return(false)

				m.store.EXPECT().CountConfigs(ctx, "prod").Return(10, nil)
				m.pdp.EXPECT().
					Has("user@example.com", domain.Permission{
						Object: domain.ObjectConfig, Action: domain.ActionWrite, Domain: "prod",
					}).
					Return(true)

				return ctx
			},
			wantLen: 1,
			wantWant: map[string]struct{ canWrite bool }{
				"prod": {canWrite: true},
			},
		},
		{
			name: "unauthorized (no claims) returns ErrUnauthorized (defence in depth)",
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "list error",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.store.EXPECT().List(ctx).Return(nil, errors.New("db error"))

				return ctx
			},
			wantErr: "list namespaces: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			got, err := svc.List(ctx, tt.params)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			require.Len(t, got.Namespaces, tt.wantLen)

			for _, ns := range got.Namespaces {
				want, ok := tt.wantWant[ns.Name]
				require.Truef(t, ok, "unexpected namespace %q in result", ns.Name)
				assert.Truef(t, ns.CanRead, "CanRead must be true on every returned namespace (got %q)", ns.Name)
				assert.Equalf(t, want.canWrite, ns.CanWrite, "CanWrite mismatch for %q", ns.Name)
			}
		})
	}
}
