package profile

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/domain"
	profile_mock "github.com/sergeyslonimsky/elara/internal/handler/v2/profile/mocks"
	profilev1 "github.com/sergeyslonimsky/elara/internal/proto/elara/profile/v1"
	internalauth "github.com/sergeyslonimsky/elara/internal/service/auth"
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

			h := New(uc, config.AuthTypeOIDC, false)

			ctx := t.Context()
			if tc.email != "" {
				ctx = internalauth.WithClaims(ctx, &internalauth.Claims{Email: tc.email, Name: "Alice"})
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

func TestProfileHandler_ChangePassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		authType   config.AuthType
		setupMocks func(uc *profile_mock.Mockusecase)
		wantErr    bool
	}{
		{
			name:     "success",
			authType: config.AuthTypeBasicAuth,
			setupMocks: func(uc *profile_mock.Mockusecase) {
				uc.EXPECT().
					ChangePassword(gomock.Any(), "", "new-password").
					Return("mock-token", nil)
			},
		},
		{
			name:     "returns error when auth type is not basic",
			authType: config.AuthTypeOIDC,
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

			ctx := internalauth.WithClaims(t.Context(), &internalauth.Claims{
				Email:                  "user@example.com",
				PasswordChangeRequired: true,
			})

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
	uc.EXPECT().Logout(gomock.Any()).Return(nil)

	h := New(uc, config.AuthTypeOIDC, false)

	resp, err := h.Logout(t.Context(), connect.NewRequest(&profilev1.LogoutRequest{}))
	require.NoError(t, err)

	cookies := resp.Header().Values(cookieHeader)
	require.Len(t, cookies, 1, "expected session-clearing cookie")
	assert.Contains(t, cookies[0], sessionCookieName)
	// MaxAge=-1 is serialized as "Max-Age=0" by net/http.
	assert.Contains(t, cookies[0], "Max-Age=0")
}
