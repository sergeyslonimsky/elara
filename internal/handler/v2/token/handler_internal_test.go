package token

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	auth2 "github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	token_mock "github.com/sergeyslonimsky/elara/internal/handler/v2/token/mocks"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
	tokenv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/token/v1"
	tokenuc "github.com/sergeyslonimsky/elara/internal/usecase/token"
)

func TestTokenHandler_CreateToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mock    func(az *token_mock.Mockauthz, uc *token_mock.Mockusecase)
		wantErr bool
	}{
		{
			name: "creates token with raw token returned",
			mock: func(az *token_mock.Mockauthz, uc *token_mock.Mockusecase) {
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectToken, domain.ActionCreate, "ns1").
					Return(nil)
				uc.EXPECT().
					Create(gomock.Any(), gomock.Any(), tokenuc.CreateInput{
						Name:       "my-token",
						Namespaces: []string{"ns1"},
						Role:       domain.RoleWriter,
					}).
					Return(&domain.Token{ID: "t1", Name: "my-token", Role: domain.RoleWriter}, "elara_secret", nil)
			},
		},
		{
			name: "forbidden on namespace returns error before usecase",
			mock: func(az *token_mock.Mockauthz, _ *token_mock.Mockusecase) {
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectToken, domain.ActionCreate, "ns1").
					Return(domain.ErrForbidden)
			},
			wantErr: true,
		},
		{
			name: "no auth context returns unauthenticated",
			mock: func(az *token_mock.Mockauthz, uc *token_mock.Mockusecase) {
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectToken, domain.ActionCreate, "ns1").
					Return(nil)
				uc.EXPECT().
					Create(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, "", domain.ErrUnauthorized)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			az := token_mock.NewMockauthz(ctrl)
			uc := token_mock.NewMockusecase(ctrl)
			tc.mock(az, uc)

			h := New(az, uc)

			ctx := auth2.WithClaims(t.Context(), &auth2.Claims{Email: "user@example.com"})
			resp, err := h.CreateToken(ctx, connect.NewRequest(&tokenv1.CreateTokenRequest{
				Name:       "my-token",
				Namespaces: []string{"ns1"},
				Permission: commonv1.PermissionAction_PERMISSION_ACTION_WRITE,
			}))

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, "elara_secret", resp.Msg.GetRawToken())
			assert.NotEmpty(t, resp.Msg.GetToken().GetId())
		})
	}
}

func TestTokenHandler_ListTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     *tokenv1.ListTokensRequest
		mock    func(uc *token_mock.Mockusecase)
		wantLen int
		wantErr bool
	}{
		{
			name: "returns tokens for issued_by filter",
			req: &tokenv1.ListTokensRequest{
				Filters: &tokenv1.ListTokensRequest_Filters{
					IssuedBy: []string{"user@example.com"},
				},
			},
			mock: func(uc *token_mock.Mockusecase) {
				uc.EXPECT().
					List(gomock.Any(), gomock.Any(), tokenuc.ListParams{
						IssuedBy: []string{"user@example.com"},
					}).
					Return(&tokenuc.ListResult{
						Tokens: []*domain.Token{{ID: "t1", IssuedBy: "user@example.com"}},
						Total:  1,
						Limit:  20,
					}, nil)
			},
			wantLen: 1,
		},
		{
			name: "no filters returns all tokens",
			req:  &tokenv1.ListTokensRequest{},
			mock: func(uc *token_mock.Mockusecase) {
				uc.EXPECT().
					List(gomock.Any(), gomock.Any(), tokenuc.ListParams{}).
					Return(&tokenuc.ListResult{
						Tokens: []*domain.Token{{ID: "t1", IssuedBy: "user@example.com"}},
						Total:  1,
						Limit:  20,
					}, nil)
			},
			wantLen: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			az := token_mock.NewMockauthz(ctrl)
			uc := token_mock.NewMockusecase(ctrl)
			tc.mock(uc)

			h := New(az, uc)

			ctx := auth2.WithClaims(t.Context(), &auth2.Claims{Email: "caller@example.com"})
			resp, err := h.ListTokens(ctx, connect.NewRequest(tc.req))

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

	tok := &domain.Token{ID: "t1", IssuedBy: "user@example.com"}

	tests := []struct {
		name     string
		id       string
		mock     func(uc *token_mock.Mockusecase)
		wantErr  bool
		wantCode connect.Code
	}{
		{
			name: "returns token by ID for owner",
			id:   "t1",
			mock: func(uc *token_mock.Mockusecase) {
				uc.EXPECT().Get(gomock.Any(), gomock.Any(), "t1").Return(tok, nil)
			},
		},
		{
			name: "forbidden for stranger",
			id:   "t1",
			mock: func(uc *token_mock.Mockusecase) {
				uc.EXPECT().Get(gomock.Any(), gomock.Any(), "t1").Return(nil, domain.ErrForbidden)
			},
			wantErr:  true,
			wantCode: connect.CodePermissionDenied,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			az := token_mock.NewMockauthz(ctrl)
			uc := token_mock.NewMockusecase(ctrl)
			tc.mock(uc)

			h := New(az, uc)

			ctx := auth2.WithClaims(t.Context(), &auth2.Claims{Email: "caller@example.com"})
			resp, err := h.GetToken(
				ctx,
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

	tests := []struct {
		name    string
		id      string
		mock    func(uc *token_mock.Mockusecase)
		wantErr bool
	}{
		{
			name: "revokes token for owner",
			id:   "t1",
			mock: func(uc *token_mock.Mockusecase) {
				uc.EXPECT().Revoke(gomock.Any(), gomock.Any(), "t1").Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			az := token_mock.NewMockauthz(ctrl)
			uc := token_mock.NewMockusecase(ctrl)
			tc.mock(uc)

			h := New(az, uc)

			ctx := auth2.WithClaims(t.Context(), &auth2.Claims{Email: "caller@example.com"})
			_, err := h.RevokeToken(
				ctx,
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
