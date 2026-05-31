package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auth2 "github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func TestService_Search(t *testing.T) {
	t.Parallel()

	results := []*domain.ConfigSummary{
		{Path: "/app/1.json", Namespace: "prod"},
		{Path: "/app/2.json", Namespace: "dev"},
		{Path: "/app/3.json", Namespace: "prod"},
	}

	tests := []struct {
		name      string
		params    config.SearchParams
		mockFunc  func(context.Context, mocks) context.Context
		errIs     error
		wantTotal int
	}{
		{
			name: "missing claims",
			params: config.SearchParams{
				Query: "app",
			},
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "empty scope returns empty result",
			params: config.SearchParams{
				Query: "app",
				Limit: 10,
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth2.WithClaims(ctx, &auth2.Claims{Email: "user@example.com"})
				m.pdp.EXPECT().
					EffectiveNamespaces("user@example.com", domain.ActionRead).
					Return(authz.NewDomainSet())

				return ctx
			},
			wantTotal: 0,
		},
		{
			name: "wildcard scope returns all",
			params: config.SearchParams{
				Query:     "app",
				Namespace: "",
				Limit:     10,
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth2.WithClaims(ctx, &auth2.Claims{Email: "user@example.com"})
				m.pdp.EXPECT().
					EffectiveNamespaces("user@example.com", domain.ActionRead).
					Return(authz.NewDomainSet("*"))
				m.storage.EXPECT().SearchByPath(ctx, "app", "").Return(results, nil)

				return ctx
			},
			wantTotal: 3,
		},
		{
			name: "explicit scope filters by namespace access",
			params: config.SearchParams{
				Query:     "app",
				Namespace: "",
				Limit:     10,
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth2.WithClaims(ctx, &auth2.Claims{Email: "user@example.com"})
				m.pdp.EXPECT().
					EffectiveNamespaces("user@example.com", domain.ActionRead).
					Return(authz.NewDomainSet("prod"))
				m.storage.EXPECT().SearchByPath(ctx, "app", "").Return(results, nil)

				return ctx
			},
			wantTotal: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			ctx := tt.mockFunc(t.Context(), m)

			got, err := svc.Search(ctx, tt.params)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTotal, got.Total)
		})
	}
}
