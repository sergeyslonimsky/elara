package v2

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	internalauth "github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

func ctxWithClaimsHandler(email string) context.Context {
	return internalauth.WithClaims(context.Background(), &internalauth.Claims{Email: email})
}

func TestTokenHandler_CreateToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		email   string
		noAuth  bool
		repoErr error
		wantErr bool
	}{
		{
			name:  "creates token with raw token returned",
			email: "user@example.com",
		},
		{
			name:    "no auth context returns unauthenticated",
			noAuth:  true,
			wantErr: true,
		},
		{
			name:    "repo error propagated",
			email:   "user@example.com",
			repoErr: errors.New("storage error"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			creator := auth_mock.NewMocktokenCreator(ctrl)

			if !tc.noAuth {
				creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(tc.repoErr)
			}

			h := NewTokenHandler(authuc.NewCreateTokenUseCase(creator), nil, nil, nil)

			ctx := context.Background()
			if !tc.noAuth {
				ctx = ctxWithClaimsHandler(tc.email)
			}

			resp, err := h.CreateToken(ctx, connect.NewRequest(&authv1.CreateTokenRequest{
				Name:       "my-token",
				Namespaces: []string{"ns1"},
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
		name      string
		userEmail string
		tokens    []*domain.PAT
		repoErr   error
		wantLen   int
		wantErr   bool
	}{
		{
			name:      "returns tokens for user",
			userEmail: "user@example.com",
			tokens:    []*domain.PAT{{ID: "t1", UserEmail: "user@example.com"}},
			wantLen:   1,
		},
		{
			name:    "returns empty list",
			tokens:  []*domain.PAT{},
			wantLen: 0,
		},
		{
			name:    "storage error propagated",
			repoErr: errors.New("storage error"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			lister := auth_mock.NewMocktokenLister(ctrl)
			lister.EXPECT().List(gomock.Any(), tc.userEmail).Return(tc.tokens, tc.repoErr)

			h := NewTokenHandler(nil, authuc.NewListTokensUseCase(lister), nil, nil)

			resp, err := h.ListTokens(context.Background(), connect.NewRequest(&authv1.ListTokensRequest{
				UserEmail: tc.userEmail,
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

	tests := []struct {
		name     string
		id       string
		token    *domain.PAT
		repoErr  error
		wantErr  bool
		wantCode connect.Code
	}{
		{
			name:  "returns token by ID",
			id:    "t1",
			token: &domain.PAT{ID: "t1", UserEmail: "user@example.com"},
		},
		{
			name:     "not found returns NotFound",
			id:       "missing",
			repoErr:  domain.NewNotFoundError("token", "missing"),
			wantErr:  true,
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			getter := auth_mock.NewMocktokenIDGetter(ctrl)
			getter.EXPECT().GetByID(gomock.Any(), tc.id).Return(tc.token, tc.repoErr)

			h := NewTokenHandler(nil, nil, authuc.NewGetTokenUseCase(getter), nil)

			resp, err := h.GetToken(context.Background(), connect.NewRequest(&authv1.GetTokenRequest{Id: tc.id}))

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

	tests := []struct {
		name    string
		id      string
		repoErr error
		wantErr bool
	}{
		{
			name: "revokes token",
			id:   "t1",
		},
		{
			name:    "not found returns error",
			id:      "missing",
			repoErr: domain.NewNotFoundError("token", "missing"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			deleter := auth_mock.NewMocktokenDeleter(ctrl)
			deleter.EXPECT().Delete(gomock.Any(), tc.id).Return(tc.repoErr)

			h := NewTokenHandler(nil, nil, nil, authuc.NewRevokeTokenUseCase(deleter))

			_, err := h.RevokeToken(context.Background(), connect.NewRequest(&authv1.RevokeTokenRequest{Id: tc.id}))

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
		})
	}
}
