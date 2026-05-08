package token

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	internalauth "github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	tokenv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/token/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

func ctxWithClaims(email string) context.Context {
	return internalauth.WithClaims(context.Background(), &internalauth.Claims{Email: email})
}

func TestTokenHandler_CreateToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		email   string
		noAuth  bool
		mock    func(enforcer *auth_mock.MocktokenEnforcer, creator *auth_mock.MocktokenCreator)
		wantErr bool
	}{
		{
			name:  "creates token with raw token returned",
			email: "user@example.com",
			mock: func(enforcer *auth_mock.MocktokenEnforcer, creator *auth_mock.MocktokenCreator) {
				enforcer.EXPECT().Enforce("user@example.com", "ns1", "namespace", "read").Return(true, nil)
				enforcer.EXPECT().Enforce("user@example.com", "ns1", "config", "write").Return(true, nil)
				creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name:    "no auth context returns unauthenticated",
			noAuth:  true,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			enforcer := auth_mock.NewMocktokenEnforcer(ctrl)
			creator := auth_mock.NewMocktokenCreator(ctrl)

			if tc.mock != nil {
				tc.mock(enforcer, creator)
			}

			h := New(authuc.NewCreateTokenUseCase(enforcer, creator), nil, nil, nil)

			ctx := context.Background()
			if !tc.noAuth {
				ctx = ctxWithClaims(tc.email)
			}

			resp, err := h.CreateToken(ctx, connect.NewRequest(&tokenv1.CreateTokenRequest{
				Name:       "my-token",
				Namespaces: []string{"ns1"},
				Role:       "writer",
			}))

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(resp.Msg.GetRawToken(), "elara_"))
			assert.NotEmpty(t, resp.Msg.GetToken().GetId())
		})
	}
}

func TestTokenHandler_ListTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		email    string
		issuedBy string
		tokens   []*domain.Token
		mock     func(enforcer *auth_mock.MocktokenEnforcer, lister *auth_mock.MocktokenLister)
		wantLen  int
		wantErr  bool
	}{
		{
			name:     "returns tokens for user",
			email:    "user@example.com",
			issuedBy: "user@example.com",
			tokens:   []*domain.Token{{ID: "t1", IssuedBy: "user@example.com"}},
			mock: func(enforcer *auth_mock.MocktokenEnforcer, lister *auth_mock.MocktokenLister) {
				enforcer.EXPECT().Enforce("user@example.com", "*", "token", "read").Return(false, nil)
				lister.EXPECT().
					List(gomock.Any(), "user@example.com").
					Return([]*domain.Token{{ID: "t1", IssuedBy: "user@example.com"}}, nil)
			},
			wantLen: 1,
		},
		{
			name:     "admin returns all tokens",
			email:    "admin@example.com",
			issuedBy: "",
			tokens:   []*domain.Token{{ID: "t1", IssuedBy: "user@example.com"}},
			mock: func(enforcer *auth_mock.MocktokenEnforcer, lister *auth_mock.MocktokenLister) {
				enforcer.EXPECT().Enforce("admin@example.com", "*", "token", "read").Return(true, nil)
				lister.EXPECT().
					List(gomock.Any(), "").
					Return([]*domain.Token{{ID: "t1", IssuedBy: "user@example.com"}}, nil)
			},
			wantLen: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			enforcer := auth_mock.NewMocktokenEnforcer(ctrl)
			lister := auth_mock.NewMocktokenLister(ctrl)

			if tc.mock != nil {
				tc.mock(enforcer, lister)
			}

			h := New(nil, authuc.NewListTokensUseCase(enforcer, lister), nil, nil)

			resp, err := h.ListTokens(ctxWithClaims(tc.email), connect.NewRequest(&tokenv1.ListTokensRequest{
				IssuedBy: tc.issuedBy,
			}))

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Len(t, resp.Msg.GetTokens(), tc.wantLen)
		})
	}
}

func TestTokenHandler_GetToken(t *testing.T) {
	t.Parallel()

	token := &domain.Token{ID: "t1", IssuedBy: "user@example.com"}

	tests := []struct {
		name     string
		email    string
		id       string
		mock     func(enforcer *auth_mock.MocktokenEnforcer, getter *auth_mock.MocktokenIDGetter)
		wantErr  bool
		wantCode connect.Code
	}{
		{
			name:  "returns token by ID for owner",
			email: "user@example.com",
			id:    "t1",
			mock: func(_ *auth_mock.MocktokenEnforcer, getter *auth_mock.MocktokenIDGetter) {
				getter.EXPECT().GetByID(gomock.Any(), "t1").Return(token, nil)
			},
		},
		{
			name:  "forbidden for stranger",
			email: "other@example.com",
			id:    "t1",
			mock: func(enforcer *auth_mock.MocktokenEnforcer, getter *auth_mock.MocktokenIDGetter) {
				getter.EXPECT().GetByID(gomock.Any(), "t1").Return(token, nil)
				enforcer.EXPECT().Enforce("other@example.com", "*", "token", "read").Return(false, nil)
			},
			wantErr:  true,
			wantCode: connect.CodePermissionDenied,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			enforcer := auth_mock.NewMocktokenEnforcer(ctrl)
			getter := auth_mock.NewMocktokenIDGetter(ctrl)

			if tc.mock != nil {
				tc.mock(enforcer, getter)
			}

			h := New(nil, nil, authuc.NewGetTokenUseCase(enforcer, getter), nil)

			resp, err := h.GetToken(
				ctxWithClaims(tc.email),
				connect.NewRequest(&tokenv1.GetTokenRequest{Id: tc.id}),
			)

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.id, resp.Msg.GetToken().GetId())
		})
	}
}

func TestTokenHandler_RevokeToken(t *testing.T) {
	t.Parallel()

	token := &domain.Token{ID: "t1", IssuedBy: "user@example.com"}

	tests := []struct {
		name    string
		email   string
		id      string
		mock    func(enforcer *auth_mock.MocktokenEnforcer, deleter *auth_mock.MocktokenDeleter, getter *auth_mock.MocktokenIDGetter)
		wantErr bool
	}{
		{
			name:  "revokes token for owner",
			email: "user@example.com",
			id:    "t1",
			mock: func(_ *auth_mock.MocktokenEnforcer, deleter *auth_mock.MocktokenDeleter, getter *auth_mock.MocktokenIDGetter) {
				getter.EXPECT().GetByID(gomock.Any(), "t1").Return(token, nil)
				deleter.EXPECT().Delete(gomock.Any(), "t1").Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			enforcer := auth_mock.NewMocktokenEnforcer(ctrl)
			deleter := auth_mock.NewMocktokenDeleter(ctrl)
			getter := auth_mock.NewMocktokenIDGetter(ctrl)

			if tc.mock != nil {
				tc.mock(enforcer, deleter, getter)
			}

			h := New(nil, nil, nil, authuc.NewRevokeTokenUseCase(enforcer, deleter, getter))

			_, err := h.RevokeToken(
				ctxWithClaims(tc.email),
				connect.NewRequest(&tokenv1.RevokeTokenRequest{Id: tc.id}),
			)

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
		})
	}
}
