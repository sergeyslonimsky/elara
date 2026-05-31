package profile

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	profile_mock "github.com/sergeyslonimsky/elara/internal/handler/v2/profile/mocks"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
	profilev1 "github.com/sergeyslonimsky/elara/internal/proto/elara/profile/v1"
	profileuc "github.com/sergeyslonimsky/elara/internal/usecase/profile"
)

func TestProfileHandler_Me(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		email    string
		ucResult *profileuc.MeResult
		ucErr    error
		wantErr  bool
		wantCode connect.Code
	}{
		{
			name:     "returns user identity",
			email:    "alice@example.com",
			ucResult: &profileuc.MeResult{Email: "alice@example.com", Name: "Alice"},
		},
		{
			name:     "no auth context returns unauthenticated",
			ucErr:    domain.ErrUnauthorized,
			wantErr:  true,
			wantCode: connect.CodeUnauthenticated,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := profile_mock.NewMockusecase(ctrl)
			uc.EXPECT().Me(gomock.Any()).Return(tc.ucResult, tc.ucErr)

			h := New(uc, domain.AuthTypeOIDC, false)

			ctx := t.Context()
			if tc.email != "" {
				ctx = authctx.WithClaims(
					ctx,
					&authctx.Claims{Email: tc.email, Name: "Alice"},
				)
			}

			resp, err := h.Me(ctx, connect.NewRequest(&profilev1.MeRequest{}))

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.email, resp.Msg.GetEmail())
		})
	}
}

func TestProfileHandler_Me_permissions_mapping(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := profile_mock.NewMockusecase(ctrl)
	uc.EXPECT().Me(gomock.Any()).Return(&profileuc.MeResult{
		Email: "alice@example.com",
		Name:  "Alice",
		Permissions: []domain.Permission{
			{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: domain.NamespaceResource("ns1")},
			{Object: domain.ObjectAll, Action: domain.ActionAll, Domain: domain.DomainAll},
		},
	}, nil)

	h := New(uc, domain.AuthTypeOIDC, false)

	ctx := authctx.WithClaims(
		t.Context(),
		&authctx.Claims{Email: "alice@example.com", Name: "Alice"},
	)

	resp, err := h.Me(ctx, connect.NewRequest(&profilev1.MeRequest{}))
	require.NoError(t, err)

	perms := resp.Msg.GetPermissions()
	require.Len(t, perms, 2)

	assert.Equal(t, commonv1.PermissionObject_PERMISSION_OBJECT_NAMESPACE, perms[0].GetObject())
	assert.Equal(t, commonv1.PermissionAction_PERMISSION_ACTION_READ, perms[0].GetAction())
	assert.Equal(t, domain.NamespaceResource("ns1"), perms[0].GetDomain())

	assert.Equal(t, commonv1.PermissionObject_PERMISSION_OBJECT_ALL, perms[1].GetObject())
	assert.Equal(t, commonv1.PermissionAction_PERMISSION_ACTION_ALL, perms[1].GetAction())
	assert.Equal(t, domain.DomainAll, perms[1].GetDomain())
}

func TestProfileHandler_ChangePassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		authType   domain.AuthType
		setupMocks func(uc *profile_mock.Mockusecase)
		wantErr    bool
	}{
		{
			name:     "success",
			authType: domain.AuthTypeBasicAuth,
			setupMocks: func(uc *profile_mock.Mockusecase) {
				uc.EXPECT().
					ChangePassword(gomock.Any(), gomock.Any()).
					Return(&domain.Session{ID: "new-session"}, nil)
			},
		},
		{
			name:     "returns error when auth type is not basic",
			authType: domain.AuthTypeOIDC,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := profile_mock.NewMockusecase(ctrl)
			if tt.setupMocks != nil {
				tt.setupMocks(uc)
			}

			h := New(uc, tt.authType, false)

			ctx := authctx.WithSession(
				t.Context(),
				&domain.Session{ID: "old-session", UserID: "user@example.com"},
				&domain.User{Email: "user@example.com", PasswordChangeRequired: true},
			)

			req := connect.NewRequest(&profilev1.ChangePasswordRequest{
				NewPassword: "new-password",
			})

			_, err := h.ChangePassword(ctx, req)

			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestProfileHandler_Logout(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := profile_mock.NewMockusecase(ctrl)
	// Expect Logout call with session ID and email
	uc.EXPECT().Logout(gomock.Any(), "old-session", "user@example.com").Return(nil)

	h := New(uc, domain.AuthTypeOIDC, false)

	ctx := authctx.WithSession(
		t.Context(),
		&domain.Session{ID: "old-session", UserID: "user@example.com"},
		&domain.User{Email: "user@example.com"},
	)

	resp, err := h.Logout(ctx, connect.NewRequest(&profilev1.LogoutRequest{}))
	require.NoError(t, err)

	cookies := resp.Header().Values("Set-Cookie")
	require.Len(t, cookies, 1, "expected session-clearing cookie")
	assert.Contains(t, cookies[0], "elara_session")
	assert.Contains(t, cookies[0], "Max-Age=0")
}
