package token_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/usecase/token"
)

func TestService_List(t *testing.T) {
	t.Parallel()

	resultTokens := []*domain.Token{
		{ID: "t1", IssuedBy: "user@example.com", Namespaces: []string{"ns1"}},
		{ID: "t2", IssuedBy: "user@example.com", Namespaces: []string{"ns2"}},
	}

	tests := []struct {
		name     string
		caller   string
		params   token.ListParams
		mockFunc func(m mocks)
		want     *token.ListResult
		errIs    error
		wantErr  string
	}{
		{
			name:   "wildcard token scope forwards AnyNamespace",
			caller: "admin@example.com",
			params: token.ListParams{},
			mockFunc: func(m mocks) {
				m.pdp.EXPECT().
					EffectiveDomains(testUserID, domain.ObjectToken, domain.ActionRead).
					Return(authz.DomainSet{Wildcard: true})
				m.store.EXPECT().List(gomock.Any(), domain.TokenFilter{
					NamespaceScopes: nil,
					AnyNamespace:    true,
				}, domain.TokenListParams{Limit: 20}).
					Return(resultTokens, 2, nil)
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
			caller: "user@example.com",
			params: token.ListParams{},
			mockFunc: func(m mocks) {
				m.pdp.EXPECT().
					EffectiveDomains(testUserID, domain.ObjectToken, domain.ActionRead).
					Return(authz.DomainSet{
						Explicit: map[string]struct{}{"ns1": {}, "ns2": {}},
					})
				m.store.EXPECT().List(gomock.Any(), domain.TokenFilter{
					NamespaceScopes: map[string]struct{}{"ns1": {}, "ns2": {}},
					AnyNamespace:    false,
				}, domain.TokenListParams{Limit: 20}).
					Return(resultTokens, 2, nil)
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
			caller: "noaccess@example.com",
			params: token.ListParams{},
			mockFunc: func(m mocks) {
				m.pdp.EXPECT().
					EffectiveDomains(testUserID, domain.ObjectToken, domain.ActionRead).
					Return(authz.DomainSet{Explicit: map[string]struct{}{}})
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
			caller: "admin@example.com",
			params: token.ListParams{IssuedBy: []string{"alice@example.com"}},
			mockFunc: func(m mocks) {
				m.pdp.EXPECT().
					EffectiveDomains(testUserID, domain.ObjectToken, domain.ActionRead).
					Return(authz.DomainSet{Wildcard: true})
				m.store.EXPECT().List(gomock.Any(), domain.TokenFilter{
					AnyNamespace: true,
					IssuedBy:     []string{"alice@example.com"},
				}, domain.TokenListParams{Limit: 20}).
					Return(resultTokens[:1], 1, nil)
			},
			want: &token.ListResult{
				Tokens: resultTokens[:1],
				Total:  1,
				Limit:  20,
				Offset: 0,
			},
		},
		{
			name:   "QueryParams forwarded as filter.QueryParams",
			caller: "admin@example.com",
			params: token.ListParams{QueryParams: []string{"prod"}},
			mockFunc: func(m mocks) {
				m.pdp.EXPECT().
					EffectiveDomains(testUserID, domain.ObjectToken, domain.ActionRead).
					Return(authz.DomainSet{Wildcard: true})
				m.store.EXPECT().List(gomock.Any(), domain.TokenFilter{
					AnyNamespace: true,
					QueryParams:  []string{"prod"},
				}, domain.TokenListParams{Limit: 20}).
					Return(resultTokens, 2, nil)
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
			caller: "admin@example.com",
			params: token.ListParams{Limit: 5, Offset: 10},
			mockFunc: func(m mocks) {
				m.pdp.EXPECT().
					EffectiveDomains(testUserID, domain.ObjectToken, domain.ActionRead).
					Return(authz.DomainSet{Wildcard: true})
				m.store.EXPECT().List(gomock.Any(), domain.TokenFilter{
					AnyNamespace: true,
				}, domain.TokenListParams{Limit: 5, Offset: 10}).
					Return([]*domain.Token{}, 12, nil)
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
			caller: "admin@example.com",
			params: token.ListParams{Limit: 0},
			mockFunc: func(m mocks) {
				m.pdp.EXPECT().
					EffectiveDomains(testUserID, domain.ObjectToken, domain.ActionRead).
					Return(authz.DomainSet{Wildcard: true})
				m.store.EXPECT().List(gomock.Any(), domain.TokenFilter{
					AnyNamespace: true,
				}, domain.TokenListParams{Limit: 20}).
					Return(resultTokens, 2, nil)
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
			caller: "admin@example.com",
			params: token.ListParams{},
			mockFunc: func(m mocks) {
				m.pdp.EXPECT().
					EffectiveDomains(testUserID, domain.ObjectToken, domain.ActionRead).
					Return(authz.DomainSet{Wildcard: true})
				m.store.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, 0, errors.New("db error"))
			},
			wantErr: "list tokens:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m := setupService(t)
			tt.mockFunc(m)

			got, err := svc.List(t.Context(), authUser(tt.caller), tt.params)

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
