package token

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"

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

func TestTokenHandler_CreateToken_NoAuthContext(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	az := token_mock.NewMockauthz(ctrl)
	uc := token_mock.NewMockusecase(ctrl)

	h := New(az, uc)

	_, err := h.CreateToken(t.Context(), connect.NewRequest(&tokenv1.CreateTokenRequest{
		Name:       "my-token",
		Namespaces: []string{"ns1"},
		Permission: commonv1.PermissionAction_PERMISSION_ACTION_WRITE,
	}))

	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestTokenHandler_CreateToken_InvalidPermission(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	az := token_mock.NewMockauthz(ctrl)
	uc := token_mock.NewMockusecase(ctrl)

	az.EXPECT().
		Require(gomock.Any(), domain.ObjectToken, domain.ActionCreate, "ns1").
		Return(nil)

	h := New(az, uc)

	ctx := auth2.WithClaims(t.Context(), &auth2.Claims{Email: "user@example.com"})
	_, err := h.CreateToken(ctx, connect.NewRequest(&tokenv1.CreateTokenRequest{
		Name:       "my-token",
		Namespaces: []string{"ns1"},
		Permission: commonv1.PermissionAction_PERMISSION_ACTION_UNSPECIFIED,
	}))

	require.ErrorContains(t, err, "permission must be read or write")
}

func TestTokenHandler_CreateToken_WithExpiresAt(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	az := token_mock.NewMockauthz(ctrl)
	uc := token_mock.NewMockusecase(ctrl)

	expires := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	az.EXPECT().
		Require(gomock.Any(), domain.ObjectToken, domain.ActionCreate, "ns1").
		Return(nil)
	uc.EXPECT().
		Create(gomock.Any(), gomock.Any(), gomock.Cond(func(in any) bool {
			ci, ok := in.(tokenuc.CreateInput)

			return ok && ci.ExpiresAt != nil && ci.ExpiresAt.Equal(expires)
		})).
		Return(&domain.Token{ID: "t1", Name: "my-token", Role: domain.RoleReader}, "raw", nil)

	h := New(az, uc)

	ctx := auth2.WithClaims(t.Context(), &auth2.Claims{Email: "user@example.com"})
	_, err := h.CreateToken(ctx, connect.NewRequest(&tokenv1.CreateTokenRequest{
		Name:       "my-token",
		Namespaces: []string{"ns1"},
		Permission: commonv1.PermissionAction_PERMISSION_ACTION_READ,
		ExpiresAt:  timestamppb.New(expires),
	}))

	require.NoError(t, err)
}

func TestTokenHandler_ListTokens_NoAuthContext(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	az := token_mock.NewMockauthz(ctrl)
	uc := token_mock.NewMockusecase(ctrl)

	h := New(az, uc)

	_, err := h.ListTokens(t.Context(), connect.NewRequest(&tokenv1.ListTokensRequest{}))

	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestTokenHandler_ListTokens_Pagination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     *tokenv1.ListTokensRequest
		mock    func(uc *token_mock.Mockusecase)
		wantErr string
	}{
		{
			name: "valid pagination and sort are forwarded to usecase",
			req: &tokenv1.ListTokensRequest{
				Pagination: &commonv1.PaginationRequest{Limit: 10, Offset: 5},
				Sorting: &commonv1.SortRequest{
					Field:     "created_at",
					Direction: commonv1.SortDirection_SORT_DIRECTION_DESC,
				},
			},
			mock: func(uc *token_mock.Mockusecase) {
				uc.EXPECT().
					List(gomock.Any(), gomock.Any(), tokenuc.ListParams{
						Limit:  10,
						Offset: 5,
						Sort:   domain.SortParams{Field: "created_at", Desc: true},
					}).
					Return(&tokenuc.ListResult{}, nil)
			},
		},
		{
			name: "negative limit returns invalid argument before calling usecase",
			req: &tokenv1.ListTokensRequest{
				Pagination: &commonv1.PaginationRequest{Limit: -1},
			},
			mock:    func(*token_mock.Mockusecase) {},
			wantErr: "normalize limit",
		},
		{
			name: "negative offset returns invalid argument before calling usecase",
			req: &tokenv1.ListTokensRequest{
				Pagination: &commonv1.PaginationRequest{Limit: 1, Offset: -1},
			},
			mock:    func(*token_mock.Mockusecase) {},
			wantErr: "normalize offset",
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
			_, err := h.ListTokens(ctx, connect.NewRequest(tc.req))

			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestTokenHandler_ListTokens_UsecaseError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	az := token_mock.NewMockauthz(ctrl)
	uc := token_mock.NewMockusecase(ctrl)
	uc.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, domain.ErrUnauthorized)

	h := New(az, uc)

	ctx := auth2.WithClaims(t.Context(), &auth2.Claims{Email: "caller@example.com"})
	_, err := h.ListTokens(ctx, connect.NewRequest(&tokenv1.ListTokensRequest{}))

	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestTokenHandler_GetToken_NoAuthContext(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	az := token_mock.NewMockauthz(ctrl)
	uc := token_mock.NewMockusecase(ctrl)

	h := New(az, uc)

	_, err := h.GetToken(t.Context(), connect.NewRequest(&tokenv1.GetTokenRequest{Id: "t1"}))

	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestTokenHandler_RevokeToken_NoAuthContext(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	az := token_mock.NewMockauthz(ctrl)
	uc := token_mock.NewMockusecase(ctrl)

	h := New(az, uc)

	_, err := h.RevokeToken(t.Context(), connect.NewRequest(&tokenv1.RevokeTokenRequest{Id: "t1"}))

	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestTokenHandler_RevokeToken_UsecaseError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	az := token_mock.NewMockauthz(ctrl)
	uc := token_mock.NewMockusecase(ctrl)
	uc.EXPECT().Revoke(gomock.Any(), gomock.Any(), "t1").Return(domain.ErrForbidden)

	h := New(az, uc)

	ctx := auth2.WithClaims(t.Context(), &auth2.Claims{Email: "caller@example.com"})
	_, err := h.RevokeToken(ctx, connect.NewRequest(&tokenv1.RevokeTokenRequest{Id: "t1"}))

	require.ErrorIs(t, err, domain.ErrForbidden)
}

func TestDomainTokenToProto(t *testing.T) {
	t.Parallel()

	expires := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	lastUsed := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	created := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		tok  *domain.Token
		want *tokenv1.Token
	}{
		{
			name: "nil token returns nil",
			tok:  nil,
			want: nil,
		},
		{
			name: "token with no expires/last-used leaves those fields unset",
			tok: &domain.Token{
				ID:        "t1",
				Name:      "n",
				IssuedBy:  "u@example.com",
				Role:      domain.RoleReader,
				CreatedAt: created,
			},
			want: &tokenv1.Token{
				Id:         "t1",
				Name:       "n",
				IssuedBy:   "u@example.com",
				Permission: commonv1.PermissionAction_PERMISSION_ACTION_READ,
				CreatedAt:  timestamppb.New(created),
			},
		},
		{
			name: "token with expires and last-used sets both timestamps",
			tok: &domain.Token{
				ID:         "t2",
				Name:       "n2",
				IssuedBy:   "u@example.com",
				Namespaces: []string{"ns1"},
				Role:       domain.RoleWriter,
				ExpiresAt:  &expires,
				LastUsedAt: &lastUsed,
				LastUsedIP: "1.2.3.4",
				CreatedAt:  created,
			},
			want: &tokenv1.Token{
				Id:         "t2",
				Name:       "n2",
				IssuedBy:   "u@example.com",
				Namespaces: []string{"ns1"},
				Permission: commonv1.PermissionAction_PERMISSION_ACTION_WRITE,
				LastUsedIp: "1.2.3.4",
				CreatedAt:  timestamppb.New(created),
				ExpiresAt:  timestamppb.New(expires),
				LastUsedAt: timestamppb.New(lastUsed),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := domainTokenToProto(tc.tok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPermissionActionToRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		action  commonv1.PermissionAction
		want    domain.Role
		wantErr string
	}{
		{
			name:   "read maps to reader",
			action: commonv1.PermissionAction_PERMISSION_ACTION_READ,
			want:   domain.RoleReader,
		},
		{
			name:   "write maps to writer",
			action: commonv1.PermissionAction_PERMISSION_ACTION_WRITE,
			want:   domain.RoleWriter,
		},
		{
			name:    "unspecified is rejected",
			action:  commonv1.PermissionAction_PERMISSION_ACTION_UNSPECIFIED,
			wantErr: "permission must be read or write",
		},
		{
			name:    "create is rejected",
			action:  commonv1.PermissionAction_PERMISSION_ACTION_CREATE,
			wantErr: "permission must be read or write",
		},
		{
			name:    "all is rejected",
			action:  commonv1.PermissionAction_PERMISSION_ACTION_ALL,
			wantErr: "permission must be read or write",
		},
		{
			name:    "unknown value is rejected",
			action:  commonv1.PermissionAction(100),
			wantErr: "unknown permission",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := permissionActionToRole(tc.action)

			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRoleToPermissionAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role domain.Role
		want commonv1.PermissionAction
	}{
		{
			name: "reader",
			role: domain.RoleReader,
			want: commonv1.PermissionAction_PERMISSION_ACTION_READ,
		},
		{
			name: "writer",
			role: domain.RoleWriter,
			want: commonv1.PermissionAction_PERMISSION_ACTION_WRITE,
		},
		{
			name: "admin",
			role: domain.RoleAdmin,
			want: commonv1.PermissionAction_PERMISSION_ACTION_ALL,
		},
		{
			name: "unknown role",
			role: domain.Role("bogus"),
			want: commonv1.PermissionAction_PERMISSION_ACTION_UNSPECIFIED,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, roleToPermissionAction(tc.role))
		})
	}
}

func TestProtoSortToDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sort *commonv1.SortRequest
		want domain.SortParams
	}{
		{
			name: "nil sort returns zero value",
			sort: nil,
			want: domain.SortParams{},
		},
		{
			name: "ascending direction",
			sort: &commonv1.SortRequest{
				Field:     "name",
				Direction: commonv1.SortDirection_SORT_DIRECTION_ASC,
			},
			want: domain.SortParams{Field: "name", Desc: false},
		},
		{
			name: "descending direction",
			sort: &commonv1.SortRequest{
				Field:     "name",
				Direction: commonv1.SortDirection_SORT_DIRECTION_DESC,
			},
			want: domain.SortParams{Field: "name", Desc: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, protoSortToDomain(tc.sort))
		})
	}
}

func TestDomainSortToProtoResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sort domain.SortParams
		want *commonv1.SortResponse
	}{
		{
			name: "empty field returns nil",
			sort: domain.SortParams{},
			want: nil,
		},
		{
			name: "ascending",
			sort: domain.SortParams{Field: "name", Desc: false},
			want: &commonv1.SortResponse{
				Field:     "name",
				Direction: commonv1.SortDirection_SORT_DIRECTION_ASC,
			},
		},
		{
			name: "descending",
			sort: domain.SortParams{Field: "name", Desc: true},
			want: &commonv1.SortResponse{
				Field:     "name",
				Direction: commonv1.SortDirection_SORT_DIRECTION_DESC,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, domainSortToProtoResponse(tc.sort))
		})
	}
}
