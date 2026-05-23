package token_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/usecase/token"
)

func TestService_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    token.CreateInput
		mockFunc func(context.Context, mocks) context.Context
		errIs    error
		wantErr  string
	}{
		{
			name: "success",
			input: token.CreateInput{
				Name:       "test-token",
				Namespaces: []string{"ns1"},
				Role:       "writer",
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = ctxWithClaims(ctx, "user@example.com")

				m.pdp.EXPECT().Has("user@example.com", domain.Permission{
					Object: domain.ObjectConfig,
					Action: domain.ActionWrite,
					Domain: "ns1",
				}).Return(true)
				m.store.EXPECT().Create(ctx, gomock.Any()).Return(nil)

				return ctx
			},
		},
		{
			name: "role-boundary denied: writer token requires config:write on ns",
			input: token.CreateInput{
				Name:       "test-token",
				Namespaces: []string{"ns1"},
				Role:       "writer",
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = ctxWithClaims(ctx, "user@example.com")

				m.pdp.EXPECT().Has("user@example.com", domain.Permission{
					Object: domain.ObjectConfig,
					Action: domain.ActionWrite,
					Domain: "ns1",
				}).Return(false)

				return ctx
			},
			errIs: domain.ErrPermissionEscalation,
		},
		{
			name: "role-boundary: reader token requires config:read on ns",
			input: token.CreateInput{
				Name:       "test-token",
				Namespaces: []string{"ns1"},
				Role:       "reader",
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = ctxWithClaims(ctx, "user@example.com")

				m.pdp.EXPECT().Has("user@example.com", domain.Permission{
					Object: domain.ObjectConfig,
					Action: domain.ActionRead,
					Domain: "ns1",
				}).Return(false)

				return ctx
			},
			errIs: domain.ErrPermissionEscalation,
		},
		{
			name: "invalid role",
			input: token.CreateInput{
				Name:       "test-token",
				Namespaces: []string{"ns1"},
				Role:       "bogus",
			},
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctxWithClaims(ctx, "user@example.com")
			},
			wantErr: "must be reader or writer",
		},
		{
			name: "unauthorized",
			input: token.CreateInput{
				Name:       "test-token",
				Namespaces: []string{"ns1"},
			},
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctx // no claims
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "empty namespaces",
			input: token.CreateInput{
				Name:       "test-token",
				Namespaces: []string{},
			},
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctxWithClaims(ctx, "user@example.com")
			},
			wantErr: "at least one namespace is required",
		},
		{
			name: "store error",
			input: token.CreateInput{
				Name:       "test-token",
				Namespaces: []string{"ns1"},
				Role:       "reader",
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = ctxWithClaims(ctx, "user@example.com")

				m.pdp.EXPECT().Has("user@example.com", domain.Permission{
					Object: domain.ObjectConfig,
					Action: domain.ActionRead,
					Domain: "ns1",
				}).Return(true)
				m.store.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db error"))

				return ctx
			},
			wantErr: "db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			testToken, rawToken, err := svc.Create(ctx, tt.input)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)

			assert.NotEmpty(t, testToken.ID)
			assert.Equal(t, "user@example.com", testToken.IssuedBy)
			assert.Equal(t, tt.input.Name, testToken.Name)
			assert.Equal(t, tt.input.Namespaces, testToken.Namespaces)
			assert.Equal(t, tt.input.Role, testToken.Role)
			assert.True(t, strings.HasPrefix(rawToken, "elara_"))
		})
	}
}

func TestService_List(t *testing.T) {
	t.Parallel()

	resultTokens := []*domain.Token{
		{ID: "t1", IssuedBy: "user@example.com", Namespaces: []string{"ns1"}},
		{ID: "t2", IssuedBy: "user@example.com", Namespaces: []string{"ns2"}},
	}

	tests := []struct {
		name     string
		params   token.ListParams
		mockFunc func(context.Context, mocks) context.Context
		want     *token.ListResult
		errIs    error
		wantErr  string
	}{
		{
			name: "unauthenticated",
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctx // no claims, no mock calls
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:   "wildcard token scope forwards AnyNamespace",
			params: token.ListParams{},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = ctxWithClaims(ctx, "admin@example.com")

				m.pdp.EXPECT().
					EffectiveDomains("admin@example.com", "token", "read").
					Return(authz.DomainSet{Wildcard: true})
				m.store.EXPECT().List(ctx, domain.TokenFilter{
					NamespaceScopes: nil,
					AnyNamespace:    true,
				}, domain.TokenListParams{Limit: 20}).
					Return(resultTokens, 2, nil)

				return ctx
			},
			want: &token.ListResult{
				Tokens: resultTokens,
				Total:  2,
				Limit:  20,
				Offset: 0,
			},
		},
		{
			name:   "explicit token scope forwards NamespaceScopes",
			params: token.ListParams{},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = ctxWithClaims(ctx, "user@example.com")

				m.pdp.EXPECT().
					EffectiveDomains("user@example.com", "token", "read").
					Return(authz.DomainSet{
						Explicit: map[string]struct{}{"ns1": {}, "ns2": {}},
					})
				m.store.EXPECT().List(ctx, domain.TokenFilter{
					NamespaceScopes: map[string]struct{}{"ns1": {}, "ns2": {}},
					AnyNamespace:    false,
				}, domain.TokenListParams{Limit: 20}).
					Return(resultTokens, 2, nil)

				return ctx
			},
			want: &token.ListResult{
				Tokens: resultTokens,
				Total:  2,
				Limit:  20,
				Offset: 0,
			},
		},
		{
			name:   "empty token scope returns empty result without calling store",
			params: token.ListParams{},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = ctxWithClaims(ctx, "noaccess@example.com")

				m.pdp.EXPECT().
					EffectiveDomains("noaccess@example.com", "token", "read").
					Return(authz.DomainSet{Explicit: map[string]struct{}{}})

				return ctx
			},
			want: &token.ListResult{
				Tokens: []*domain.Token{},
				Total:  0,
				Limit:  20,
				Offset: 0,
			},
		},
		{
			name:   "IssuedBy forwarded to filter",
			params: token.ListParams{IssuedBy: "alice@example.com"},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = ctxWithClaims(ctx, "admin@example.com")

				m.pdp.EXPECT().
					EffectiveDomains("admin@example.com", "token", "read").
					Return(authz.DomainSet{Wildcard: true})
				m.store.EXPECT().List(ctx, domain.TokenFilter{
					AnyNamespace: true,
					IssuedBy:     "alice@example.com",
				}, domain.TokenListParams{Limit: 20}).
					Return(resultTokens[:1], 1, nil)

				return ctx
			},
			want: &token.ListResult{
				Tokens: resultTokens[:1],
				Total:  1,
				Limit:  20,
				Offset: 0,
			},
		},
		{
			name:   "Query forwarded as filter.Search",
			params: token.ListParams{Query: "prod"},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = ctxWithClaims(ctx, "admin@example.com")

				m.pdp.EXPECT().
					EffectiveDomains("admin@example.com", "token", "read").
					Return(authz.DomainSet{Wildcard: true})
				m.store.EXPECT().List(ctx, domain.TokenFilter{
					AnyNamespace: true,
					Search:       "prod",
				}, domain.TokenListParams{Limit: 20}).
					Return(resultTokens, 2, nil)

				return ctx
			},
			want: &token.ListResult{
				Tokens: resultTokens,
				Total:  2,
				Limit:  20,
				Offset: 0,
			},
		},
		{
			name:   "pagination forwarded",
			params: token.ListParams{Limit: 5, Offset: 10},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = ctxWithClaims(ctx, "admin@example.com")

				m.pdp.EXPECT().
					EffectiveDomains("admin@example.com", "token", "read").
					Return(authz.DomainSet{Wildcard: true})
				m.store.EXPECT().List(ctx, domain.TokenFilter{
					AnyNamespace: true,
				}, domain.TokenListParams{Limit: 5, Offset: 10}).
					Return([]*domain.Token{}, 12, nil)

				return ctx
			},
			want: &token.ListResult{
				Tokens: []*domain.Token{},
				Total:  12,
				Limit:  5,
				Offset: 10,
			},
		},
		{
			name:   "default limit when params.Limit is zero",
			params: token.ListParams{Limit: 0},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = ctxWithClaims(ctx, "admin@example.com")

				m.pdp.EXPECT().
					EffectiveDomains("admin@example.com", "token", "read").
					Return(authz.DomainSet{Wildcard: true})
				m.store.EXPECT().List(ctx, domain.TokenFilter{
					AnyNamespace: true,
				}, domain.TokenListParams{Limit: 20}).
					Return(resultTokens, 2, nil)

				return ctx
			},
			want: &token.ListResult{
				Tokens: resultTokens,
				Total:  2,
				Limit:  20,
				Offset: 0,
			},
		},
		{
			name:   "store error wrapped",
			params: token.ListParams{},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = ctxWithClaims(ctx, "admin@example.com")

				m.pdp.EXPECT().
					EffectiveDomains("admin@example.com", "token", "read").
					Return(authz.DomainSet{Wildcard: true})
				m.store.EXPECT().List(ctx, gomock.Any(), gomock.Any()).
					Return(nil, 0, errors.New("db error"))

				return ctx
			},
			wantErr: "list tokens:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m := setupService(t)
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
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestService_Get(t *testing.T) {
	t.Parallel()

	testToken := &domain.Token{ID: "t1", IssuedBy: "owner@example.com", Namespaces: []string{"ns1"}}

	tests := []struct {
		name     string
		id       string
		mockFunc func(context.Context, mocks) context.Context
		errIs    error
		wantErr  string
	}{
		{
			name: "caller with token read on scoped namespace",
			id:   "t1",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = ctxWithClaims(ctx, "user@example.com")

				m.store.EXPECT().GetByID(ctx, "t1").Return(testToken, nil)
				m.pdp.EXPECT().Has("user@example.com", domain.Permission{
					Object: domain.ObjectToken,
					Action: domain.ActionRead,
					Domain: "ns1",
				}).Return(true)

				return ctx
			},
		},
		{
			name: "forbidden when no token read on any scoped namespace",
			id:   "t1",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = ctxWithClaims(ctx, "stranger@example.com")

				m.store.EXPECT().GetByID(ctx, "t1").Return(testToken, nil)
				m.pdp.EXPECT().Has("stranger@example.com", domain.Permission{
					Object: domain.ObjectToken,
					Action: domain.ActionRead,
					Domain: "ns1",
				}).Return(false)

				return ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "unauthenticated",
			id:   "t1",
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctx
			},
			errIs: domain.ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			got, err := svc.Get(ctx, tt.id)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.id, got.ID)
		})
	}
}

func TestService_Revoke(t *testing.T) {
	t.Parallel()

	testToken := &domain.Token{ID: "t1", IssuedBy: "owner@example.com", Namespaces: []string{"ns1"}}

	tests := []struct {
		name     string
		id       string
		mockFunc func(context.Context, mocks) context.Context
		errIs    error
		wantErr  string
	}{
		{
			name: "caller with token write on scoped namespace",
			id:   "t1",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = ctxWithClaims(ctx, "admin@example.com")

				m.store.EXPECT().GetByID(ctx, "t1").Return(testToken, nil)
				m.pdp.EXPECT().Has("admin@example.com", domain.Permission{
					Object: domain.ObjectToken,
					Action: domain.ActionWrite,
					Domain: "ns1",
				}).Return(true)
				m.store.EXPECT().Delete(ctx, "t1").Return(nil)

				return ctx
			},
		},
		{
			name: "forbidden when no token write on any scoped namespace",
			id:   "t1",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = ctxWithClaims(ctx, "stranger@example.com")

				m.store.EXPECT().GetByID(ctx, "t1").Return(testToken, nil)
				m.pdp.EXPECT().Has("stranger@example.com", domain.Permission{
					Object: domain.ObjectToken,
					Action: domain.ActionWrite,
					Domain: "ns1",
				}).Return(false)

				return ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "unauthenticated",
			id:   "t1",
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctx
			},
			errIs: domain.ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			err := svc.Revoke(ctx, tt.id)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}
