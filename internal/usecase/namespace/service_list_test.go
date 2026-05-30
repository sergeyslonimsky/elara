package namespace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/usecase/namespace"
)

// List is authenticated-only at the handler boundary; the usecase pre-filters
// via pdp.EffectiveDomains and forwards a NamespaceFilter to the repo (no
// post-fetch pdp.Has loop). Per-namespace CanWrite is annotated on the result
// for the UI.

func TestService_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		params   namespace.ListParams
		mockFunc func(ctx context.Context, m mocks) context.Context
		errIs    error
		wantErr  string
		want     *namespace.ListResult
		wantWant map[string]bool // ns name -> CanWrite expectation
	}{
		{
			name: "unauthenticated returns ErrUnauthorized",
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:   "wildcard scope returns all namespaces from repo",
			params: namespace.ListParams{Limit: 10, Offset: 0},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				m.pdp.EXPECT().
					EffectiveDomains("admin@example.com", domain.ObjectNamespace, domain.ActionRead).
					Return(authz.NewDomainSet("*"))

				expectedFilter := domain.NamespaceFilter{
					Names:    map[string]struct{}{},
					Wildcard: true,
					Search:   "",
				}
				expectedParams := domain.NamespaceListParams{Limit: 10, Offset: 0}

				m.store.EXPECT().
					List(ctx, expectedFilter, expectedParams).
					Return([]*domain.Namespace{{Name: "dev"}, {Name: "prod"}}, 2, nil)

				m.store.EXPECT().CountConfigs(ctx, "dev").Return(3, nil)
				m.store.EXPECT().CountConfigs(ctx, "prod").Return(7, nil)

				m.pdp.EXPECT().
					Has("admin@example.com", domain.Permission{
						Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "dev",
					}).
					Return(true)
				m.pdp.EXPECT().
					Has("admin@example.com", domain.Permission{
						Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "prod",
					}).
					Return(false)

				return ctx
			},
			want: &namespace.ListResult{
				Total:  2,
				Limit:  10,
				Offset: 0,
			},
			wantWant: map[string]bool{"dev": true, "prod": false},
		},
		{
			name:   "explicit scope forwards Names into filter",
			params: namespace.ListParams{Limit: 5},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				m.pdp.EXPECT().
					EffectiveDomains("user@example.com", domain.ObjectNamespace, domain.ActionRead).
					Return(authz.NewDomainSet("ns1", "ns3"))

				expectedFilter := domain.NamespaceFilter{
					Names:    map[string]struct{}{"ns1": {}, "ns3": {}},
					Wildcard: false,
					Search:   "",
				}
				expectedParams := domain.NamespaceListParams{Limit: 5, Offset: 0}

				m.store.EXPECT().
					List(ctx, expectedFilter, expectedParams).
					Return([]*domain.Namespace{{Name: "ns1"}, {Name: "ns3"}}, 2, nil)

				m.store.EXPECT().CountConfigs(ctx, "ns1").Return(1, nil)
				m.store.EXPECT().CountConfigs(ctx, "ns3").Return(2, nil)

				m.pdp.EXPECT().
					Has("user@example.com", domain.Permission{
						Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "ns1",
					}).
					Return(true)
				m.pdp.EXPECT().
					Has("user@example.com", domain.Permission{
						Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "ns3",
					}).
					Return(false)

				return ctx
			},
			want: &namespace.ListResult{
				Total:  2,
				Limit:  5,
				Offset: 0,
			},
			wantWant: map[string]bool{"ns1": true, "ns3": false},
		},
		{
			name:   "empty scope returns empty list without calling store",
			params: namespace.ListParams{Limit: 10, Offset: 0},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "noaccess@example.com"})

				m.pdp.EXPECT().
					EffectiveDomains("noaccess@example.com", domain.ObjectNamespace, domain.ActionRead).
					Return(authz.NewDomainSet())

				// store.List and annotations MUST NOT be called.

				return ctx
			},
			want: &namespace.ListResult{
				Namespaces: []*domain.Namespace{},
				Total:      0,
				Limit:      10,
				Offset:     0,
			},
			wantWant: map[string]bool{},
		},
		{
			name:   "search query is forwarded to filter",
			params: namespace.ListParams{Limit: 10, Query: "prod"},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "u@example.com"})

				m.pdp.EXPECT().
					EffectiveDomains("u@example.com", domain.ObjectNamespace, domain.ActionRead).
					Return(authz.NewDomainSet("*"))

				expectedFilter := domain.NamespaceFilter{
					Names:    map[string]struct{}{},
					Wildcard: true,
					Search:   "prod",
				}
				expectedParams := domain.NamespaceListParams{Limit: 10, Offset: 0}

				m.store.EXPECT().
					List(ctx, expectedFilter, expectedParams).
					Return([]*domain.Namespace{{Name: "prod"}}, 1, nil)

				m.store.EXPECT().CountConfigs(ctx, "prod").Return(0, nil)
				m.pdp.EXPECT().
					Has("u@example.com", domain.Permission{
						Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "prod",
					}).
					Return(true)

				return ctx
			},
			want: &namespace.ListResult{
				Total:  1,
				Limit:  10,
				Offset: 0,
			},
			wantWant: map[string]bool{"prod": true},
		},
		{
			name:   "pagination params forwarded to repo",
			params: namespace.ListParams{Limit: 5, Offset: 10},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "u@example.com"})

				m.pdp.EXPECT().
					EffectiveDomains("u@example.com", domain.ObjectNamespace, domain.ActionRead).
					Return(authz.NewDomainSet("*"))

				expectedFilter := domain.NamespaceFilter{
					Names:    map[string]struct{}{},
					Wildcard: true,
					Search:   "",
				}
				expectedParams := domain.NamespaceListParams{Limit: 5, Offset: 10}

				m.store.EXPECT().
					List(ctx, expectedFilter, expectedParams).
					Return([]*domain.Namespace{}, 42, nil)

				return ctx
			},
			want: &namespace.ListResult{
				Namespaces: []*domain.Namespace{},
				Total:      42,
				Limit:      5,
				Offset:     10,
			},
			wantWant: map[string]bool{},
		},
		{
			name:   "default limit applied when params.Limit <= 0",
			params: namespace.ListParams{Limit: 0, Offset: 0},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "u@example.com"})

				m.pdp.EXPECT().
					EffectiveDomains("u@example.com", domain.ObjectNamespace, domain.ActionRead).
					Return(authz.NewDomainSet("*"))

				expectedFilter := domain.NamespaceFilter{
					Names:    map[string]struct{}{},
					Wildcard: true,
					Search:   "",
				}
				// defaultListLimit = 20 (private constant; locked in by test).
				expectedParams := domain.NamespaceListParams{Limit: 20, Offset: 0}

				m.store.EXPECT().
					List(ctx, expectedFilter, expectedParams).
					Return([]*domain.Namespace{}, 0, nil)

				return ctx
			},
			want: &namespace.ListResult{
				Namespaces: []*domain.Namespace{},
				Total:      0,
				Limit:      20,
				Offset:     0,
			},
			wantWant: map[string]bool{},
		},
		{
			name:   "store error wrapped",
			params: namespace.ListParams{Limit: 10},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "u@example.com"})

				m.pdp.EXPECT().
					EffectiveDomains("u@example.com", domain.ObjectNamespace, domain.ActionRead).
					Return(authz.NewDomainSet("*"))

				m.store.EXPECT().
					List(ctx, gomock.Any(), gomock.Any()).
					Return(nil, 0, errors.New("db error"))

				return ctx
			},
			wantErr: "list namespaces: db error",
		},
		{
			name:   "populateConfigCounts error propagated",
			params: namespace.ListParams{Limit: 10},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "u@example.com"})

				m.pdp.EXPECT().
					EffectiveDomains("u@example.com", domain.ObjectNamespace, domain.ActionRead).
					Return(authz.NewDomainSet("*"))

				m.store.EXPECT().
					List(ctx, gomock.Any(), gomock.Any()).
					Return([]*domain.Namespace{{Name: "ns1"}}, 1, nil)

				m.store.EXPECT().CountConfigs(ctx, "ns1").Return(0, errors.New("count failure"))

				return ctx
			},
			wantErr: "count configs: count failure",
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
			require.NotNil(t, got)

			assert.Equal(t, tt.want.Total, got.Total, "Total mismatch")
			assert.Equal(t, tt.want.Limit, got.Limit, "Limit mismatch")
			assert.Equal(t, tt.want.Offset, got.Offset, "Offset mismatch")

			assert.Len(t, got.Namespaces, len(tt.wantWant))
			for _, ns := range got.Namespaces {
				wantWrite, ok := tt.wantWant[ns.Name]
				require.Truef(t, ok, "unexpected namespace %q in result", ns.Name)
				assert.Truef(t, ns.CanRead, "CanRead must be true for %q", ns.Name)
				assert.Equalf(t, wantWrite, ns.CanWrite, "CanWrite mismatch for %q", ns.Name)
			}
		})
	}
}
