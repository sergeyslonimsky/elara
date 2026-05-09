package token

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	token_mock "github.com/sergeyslonimsky/elara/internal/handler/v2/token/mocks"
	tokenv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/token/v1"
	tokenuc "github.com/sergeyslonimsky/elara/internal/usecase/token"
)

func TestTokenHandler_CreateToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mock    func(uc *token_mock.Mockusecase)
		wantErr bool
	}{
		{
			name: "creates token with raw token returned",
			mock: func(uc *token_mock.Mockusecase) {
				uc.EXPECT().
					Create(gomock.Any(), tokenuc.CreateInput{
						Name:       "my-token",
						Namespaces: []string{"ns1"},
						Role:       "writer",
					}).
					Return(&domain.Token{ID: "t1", Name: "my-token"}, "elara_secret", nil)
			},
		},
		{
			name: "no auth context returns unauthenticated",
			mock: func(uc *token_mock.Mockusecase) {
				uc.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(nil, "", domain.ErrUnauthorized)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := token_mock.NewMockusecase(ctrl)
			tc.mock(uc)

			h := New(uc)

			resp, err := h.CreateToken(t.Context(), connect.NewRequest(&tokenv1.CreateTokenRequest{
				Name:       "my-token",
				Namespaces: []string{"ns1"},
				Role:       "writer",
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
		name     string
		issuedBy string
		mock     func(uc *token_mock.Mockusecase)
		wantLen  int
		wantErr  bool
	}{
		{
			name:     "returns tokens for user",
			issuedBy: "user@example.com",
			mock: func(uc *token_mock.Mockusecase) {
				uc.EXPECT().
					List(gomock.Any(), "user@example.com").
					Return([]*domain.Token{{ID: "t1", IssuedBy: "user@example.com"}}, nil)
			},
			wantLen: 1,
		},
		{
			name:     "admin returns all tokens",
			issuedBy: "",
			mock: func(uc *token_mock.Mockusecase) {
				uc.EXPECT().
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
			uc := token_mock.NewMockusecase(ctrl)
			tc.mock(uc)

			h := New(uc)

			resp, err := h.ListTokens(t.Context(), connect.NewRequest(&tokenv1.ListTokensRequest{
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
				uc.EXPECT().Get(gomock.Any(), "t1").Return(tok, nil)
			},
		},
		{
			name: "forbidden for stranger",
			id:   "t1",
			mock: func(uc *token_mock.Mockusecase) {
				uc.EXPECT().Get(gomock.Any(), "t1").Return(nil, domain.ErrForbidden)
			},
			wantErr:  true,
			wantCode: connect.CodePermissionDenied,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := token_mock.NewMockusecase(ctrl)
			tc.mock(uc)

			h := New(uc)

			resp, err := h.GetToken(
				t.Context(),
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
				uc.EXPECT().Revoke(gomock.Any(), "t1").Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := token_mock.NewMockusecase(ctrl)
			tc.mock(uc)

			h := New(uc)

			_, err := h.RevokeToken(
				t.Context(),
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
