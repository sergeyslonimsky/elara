package auth_test

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/handler/v2/auth/mocks"
	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
)

func setupHandler(
	t *testing.T,
	ctrl *gomock.Controller,
	authType domain.AuthType,
) (*auth.Handler, *auth_mock.Mockusecase) {
	t.Helper()

	uc := auth_mock.NewMockusecase(ctrl)

	return auth.NewHandler(uc, authType, false), uc
}

func TestAuthHandler_BasicLogin(t *testing.T) {
	t.Parallel()

	email := "user@example.com"
	password := "correct-password"

	tests := []struct {
		name       string
		authType   domain.AuthType
		setupMocks func(uc *auth_mock.Mockusecase)
		wantErr    bool
	}{
		{
			name:     "success sets session cookie",
			authType: domain.AuthTypeBasicAuth,
			setupMocks: func(uc *auth_mock.Mockusecase) {
				uc.EXPECT().
					BasicLogin(gomock.Any(), gomock.Any()).
					Return(&domain.User{Email: email}, &domain.Session{ID: "session-id-1"}, nil)
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
			h, uc := setupHandler(t, ctrl, tt.authType)

			if tt.setupMocks != nil {
				tt.setupMocks(uc)
			}

			req := connect.NewRequest(&authv1.BasicLoginRequest{
				Email:    email,
				Password: password,
			})

			resp, err := h.BasicLogin(t.Context(), req)

			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			cookies := resp.Header().Values("Set-Cookie")
			found := false
			for _, c := range cookies {
				if strings.Contains(c, "elara_session=session-id-1") {
					found = true

					break
				}
			}
			assert.True(t, found, "expected session cookie in response")
		})
	}
}

func TestAuthHandler_Login(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		authURL string
		wantErr bool
	}{
		{
			name:    "returns redirect URL and sets cookies",
			authURL: "https://idp.example.com/authorize?state=abc&nonce=xyz",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, uc := setupHandler(t, ctrl, domain.AuthTypeOIDC)

			uc.EXPECT().Login(gomock.Any()).Return(tc.authURL, "state-val", "nonce-val", nil)

			resp, err := h.OIDCLogin(t.Context(), connect.NewRequest(&authv1.OIDCLoginRequest{}))

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.authURL, resp.Msg.GetRedirectUrl())

			cookies := resp.Header().Values("Set-Cookie")
			assert.Len(t, cookies, 2, "expected state and nonce cookies")

			// Regression: state/nonce cookies must use Path=/ so the browser
			// sends them on the ConnectRPC callback POST to
			// /elara.auth.v1.AuthService/OIDCCallback (which is not under /auth).
			for _, c := range cookies {
				assert.Contains(t, c, "Path=/",
					"OIDC bootstrap cookies must be Path=/ so they reach the ConnectRPC callback endpoint")
			}
		})
	}
}

func TestAuthHandler_GetAuthInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		authType domain.AuthType
		expected authv1.AuthType
	}{
		{
			name:     "reports OIDC",
			authType: domain.AuthTypeOIDC,
			expected: authv1.AuthType_AUTH_TYPE_OIDC,
		},
		{
			name:     "reports basic-auth",
			authType: domain.AuthTypeBasicAuth,
			expected: authv1.AuthType_AUTH_TYPE_BASIC,
		},
		{
			name:     "reports none",
			authType: domain.AuthTypeNone,
			expected: authv1.AuthType_AUTH_TYPE_NONE,
		},
		{
			name:     "reports unspecified for unknown type",
			authType: "invalid",
			expected: authv1.AuthType_AUTH_TYPE_UNSPECIFIED,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h, _ := setupHandler(t, gomock.NewController(t), tc.authType)
			resp, err := h.GetAuthInfo(
				t.Context(),
				connect.NewRequest(&authv1.GetAuthInfoRequest{}),
			)

			require.NoError(t, err)
			assert.Equal(t, tc.expected, resp.Msg.GetAuthType())
		})
	}
}

func TestAuthHandler_Callback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		requestState  string
		stateCookie   string
		nonceCookie   string
		setupMocks    func(uc *auth_mock.Mockusecase)
		wantErr       bool
		wantCode      connect.Code
		wantMsgPart   string
		verifyCookies bool
	}{
		{
			name:          "happy path: valid state cookie sets session cookie",
			requestState:  "test-state",
			stateCookie:   "test-state",
			nonceCookie:   "test-nonce",
			verifyCookies: true,
			setupMocks: func(uc *auth_mock.Mockusecase) {
				uc.EXPECT().
					Callback(gomock.Any(), gomock.Any()).
					Return(&domain.User{Email: "user@example.com"}, &domain.Session{ID: "session-cb"}, nil)
			},
		},
		{
			name:         "state mismatch returns CodeUnauthenticated",
			requestState: "correct-state",
			stateCookie:  "wrong-state",
			nonceCookie:  "nonce-val",
			wantErr:      true,
			wantCode:     connect.CodeUnauthenticated,
		},
		{
			name:         "missing state cookie returns CodeUnauthenticated",
			requestState: "some-state",
			stateCookie:  "",
			nonceCookie:  "nonce-val",
			wantErr:      true,
			wantCode:     connect.CodeUnauthenticated,
		},
		{
			name:         "missing nonce cookie returns CodeUnauthenticated",
			requestState: "test-state",
			stateCookie:  "test-state",
			nonceCookie:  "",
			wantErr:      true,
			wantCode:     connect.CodeUnauthenticated,
		},
		{
			name:         "callback exchange error maps to connect error",
			requestState: "test-state",
			stateCookie:  "test-state",
			nonceCookie:  "test-nonce",
			wantErr:      true,
			setupMocks: func(uc *auth_mock.Mockusecase) {
				uc.EXPECT().
					Callback(gomock.Any(), gomock.Any()).
					Return(nil, nil, errors.New("provider error"))
			},
		},
		{
			// Regression for the EL-OIDC structured-errors fix: unprovisioned
			// users must see a stable user-facing string in the SPA, not the
			// raw "identity not provisioned" sentinel.
			name:         "ErrIdentityNotProvisioned returns permission_denied with stable account-not-provisioned message",
			requestState: "test-state",
			stateCookie:  "test-state",
			nonceCookie:  "test-nonce",
			wantErr:      true,
			wantCode:     connect.CodePermissionDenied,
			wantMsgPart:  "Account not provisioned",
			setupMocks: func(uc *auth_mock.Mockusecase) {
				uc.EXPECT().
					Callback(gomock.Any(), gomock.Any()).
					Return(nil, nil, fmt.Errorf("oidc callback tx: %w", domain.ErrIdentityNotProvisioned))
			},
		},
		{
			name:         "ErrUserDeactivated returns permission_denied with stable account-deactivated message",
			requestState: "test-state",
			stateCookie:  "test-state",
			nonceCookie:  "test-nonce",
			wantErr:      true,
			wantCode:     connect.CodePermissionDenied,
			wantMsgPart:  "Account deactivated",
			setupMocks: func(uc *auth_mock.Mockusecase) {
				uc.EXPECT().
					Callback(gomock.Any(), gomock.Any()).
					Return(nil, nil, fmt.Errorf("oidc callback tx: %w", domain.ErrUserDeactivated))
			},
		},
		{
			// oauth2 "invalid_grant" surfaces when the authorization code is
			// reused or expired. Must map to failed_precondition so the SPA can
			// auto-redirect back to /login instead of showing a 500.
			name:         "oauth2 invalid_grant returns failed_precondition with authorization-expired message",
			requestState: "test-state",
			stateCookie:  "test-state",
			nonceCookie:  "test-nonce",
			wantErr:      true,
			wantCode:     connect.CodeFailedPrecondition,
			wantMsgPart:  "Authorization expired",
			setupMocks: func(uc *auth_mock.Mockusecase) {
				uc.EXPECT().
					Callback(gomock.Any(), gomock.Any()).
					Return(nil, nil, errors.New(
						`oidc callback tx: exchange code: oauth2: cannot fetch token: 400 Bad Request {"error":"invalid_grant"}`,
					))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, uc := setupHandler(t, ctrl, domain.AuthTypeOIDC)

			if tt.setupMocks != nil {
				tt.setupMocks(uc)
			}

			req := connect.NewRequest(&authv1.OIDCCallbackRequest{
				State: tt.requestState,
				Code:  "auth-code",
			})

			var cookieParts []string
			if tt.stateCookie != "" {
				cookieParts = append(cookieParts, (&http.Cookie{
					Name:  "elara_oauth_state",
					Value: tt.stateCookie,
				}).String())
			}
			if tt.nonceCookie != "" {
				cookieParts = append(cookieParts, (&http.Cookie{
					Name:  "elara_oauth_nonce",
					Value: tt.nonceCookie,
				}).String())
			}
			if len(cookieParts) > 0 {
				req.Header().Set("Cookie", strings.Join(cookieParts, "; "))
			}

			resp, err := h.OIDCCallback(t.Context(), req)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantCode != 0 {
					assert.Equal(t, tt.wantCode, connect.CodeOf(err))
				}
				if tt.wantMsgPart != "" {
					assert.Contains(t, err.Error(), tt.wantMsgPart,
						"OIDC callback error must surface stable user-facing message")
				}

				return
			}

			require.NoError(t, err)

			if tt.verifyCookies {
				setCookies := resp.Header().Values("Set-Cookie")
				require.NotEmpty(t, setCookies, "expected Set-Cookie header in response")

				found := false
				for _, c := range setCookies {
					if strings.Contains(c, "elara_session") {
						found = true

						break
					}
				}
				assert.True(t, found, "elara_session cookie should be set in response")
			}
		})
	}
}
