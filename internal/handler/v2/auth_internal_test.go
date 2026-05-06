package v2

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	internalauth "github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/domain"
	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

// stubEnforcer is a minimal meEnforcer stub for handler tests.
type stubEnforcer struct {
	allowAll bool
}

func (s *stubEnforcer) Enforce(_, _, _, _ string) (bool, error) {
	return s.allowAll, nil
}

// stubNamespaceLister returns a fixed list of namespaces.
type stubNamespaceLister struct {
	namespaces []*domain.Namespace
}

func (s *stubNamespaceLister) List(_ context.Context) ([]*domain.Namespace, error) {
	return s.namespaces, nil
}

func TestAuthHandler_BasicLogin(t *testing.T) {
	t.Parallel()

	email := "user@example.com"
	password := "correct-password"
	hash, _ := internalauth.HashPassword(password)

	tests := []struct {
		name       string
		authType   config.AuthType
		setupMocks func(
			users *auth_mock.MockbasicAuthUserGetter,
			session *auth_mock.MocksessionCreator,
		)
		wantErr bool
	}{
		{
			name:     "success sets session cookie",
			authType: config.AuthTypeBasicAuth,
			setupMocks: func(users *auth_mock.MockbasicAuthUserGetter, session *auth_mock.MocksessionCreator) {
				users.EXPECT().Get(gomock.Any(), email).Return(&domain.User{Email: email, PasswordHash: hash}, nil)
				session.EXPECT().Create(gomock.Any()).Return("signed-token", nil)
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
			users := auth_mock.NewMockbasicAuthUserGetter(ctrl)
			session := auth_mock.NewMocksessionCreator(ctrl)

			if tt.setupMocks != nil {
				tt.setupMocks(users, session)
			}

			basicUC := authuc.NewBasicLoginUseCase(users, session, nil, "")
			h := NewAuthHandler(nil, nil, nil, basicUC, nil, tt.authType)

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
			cookies := resp.Header().Values(cookieHeader)
			found := false
			for _, c := range cookies {
				if strings.Contains(c, sessionCookieName+"=signed-token") {
					found = true

					break
				}
			}
			assert.True(t, found, "expected session cookie in response")
		})
	}
}

func TestAuthHandler_ChangePassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		authType   config.AuthType
		setupMocks func(
			reader *auth_mock.MockpasswordReader,
			writer *auth_mock.MockpasswordWriter,
		)
		wantErr bool
	}{
		{
			name:     "success",
			authType: config.AuthTypeBasicAuth,
			setupMocks: func(reader *auth_mock.MockpasswordReader, writer *auth_mock.MockpasswordWriter) {
				reader.EXPECT().
					Get(gomock.Any(), "user@example.com").
					Return(&domain.User{Email: "user@example.com"}, nil)
				writer.EXPECT().SetPassword(gomock.Any(), "user@example.com", gomock.Any(), false).Return(nil)
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
			reader := auth_mock.NewMockpasswordReader(ctrl)
			writer := auth_mock.NewMockpasswordWriter(ctrl)

			if tt.setupMocks != nil {
				tt.setupMocks(reader, writer)
			}

			changeUC := authuc.NewChangePasswordUseCase(reader, writer)
			h := NewAuthHandler(nil, nil, nil, nil, changeUC, tt.authType)

			ctx := internalauth.WithClaims(t.Context(), &internalauth.Claims{
				Email:                  "user@example.com",
				PasswordChangeRequired: true,
			})

			req := connect.NewRequest(&authv1.ChangePasswordRequest{
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

func newTestAuthHandler(
	loginUC *authuc.LoginUseCase,
	callbackUC *authuc.CallbackUseCase,
	meUC *authuc.MeUseCase,
) *AuthHandler {
	return NewAuthHandler(loginUC, callbackUC, meUC, nil, nil, config.AuthTypeOIDC)
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
			provider := auth_mock.NewMockoidcProvider(ctrl)
			provider.EXPECT().AuthURL(gomock.Any(), gomock.Any()).Return(tc.authURL)

			loginUC := authuc.NewLoginUseCase(provider)
			h := newTestAuthHandler(loginUC, nil, nil)

			resp, err := h.OIDCLogin(context.Background(), connect.NewRequest(&authv1.OIDCLoginRequest{}))

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.authURL, resp.Msg.GetRedirectUrl())

			cookies := resp.Header().Values(cookieHeader)
			assert.Len(t, cookies, 2, "expected state and nonce cookies")
		})
	}
}

func TestAuthHandler_GetAuthInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		authType config.AuthType
		expected authv1.AuthType
	}{
		{
			name:     "reports OIDC",
			authType: config.AuthTypeOIDC,
			expected: authv1.AuthType_AUTH_TYPE_OIDC,
		},
		{
			name:     "reports basic-auth",
			authType: config.AuthTypeBasicAuth,
			expected: authv1.AuthType_AUTH_TYPE_BASIC,
		},
		{
			name:     "reports none",
			authType: config.AuthTypeNone,
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

			h := NewAuthHandler(nil, nil, nil, nil, nil, tc.authType)
			resp, err := h.GetAuthInfo(context.Background(), connect.NewRequest(&authv1.GetAuthInfoRequest{}))

			require.NoError(t, err)
			assert.Equal(t, tc.expected, resp.Msg.GetAuthType())
		})
	}
}

func TestAuthHandler_Logout(t *testing.T) {
	t.Parallel()

	h := NewAuthHandler(nil, nil, nil, nil, nil, config.AuthTypeOIDC)

	resp, err := h.Logout(context.Background(), connect.NewRequest(&authv1.LogoutRequest{}))
	require.NoError(t, err)

	cookies := resp.Header().Values(cookieHeader)
	require.Len(t, cookies, 1, "expected session-clearing cookie")
	assert.Contains(t, cookies[0], sessionCookieName)
	// MaxAge=-1 is serialized as "Max-Age=0" by net/http.
	assert.Contains(t, cookies[0], "Max-Age=0")
}

func TestAuthHandler_Me(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		email    string
		authCtx  bool
		wantErr  bool
		wantCode connect.Code
	}{
		{
			name:    "returns user identity",
			email:   "alice@example.com",
			authCtx: true,
		},
		{
			name:     "no auth context returns unauthenticated",
			authCtx:  false,
			wantErr:  true,
			wantCode: connect.CodeUnauthenticated,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			enforcer := &stubEnforcer{allowAll: false}
			nsList := &stubNamespaceLister{namespaces: nil}
			meUC := authuc.NewMeUseCase(enforcer, nsList)
			h := newTestAuthHandler(nil, nil, meUC)

			ctx := context.Background()
			if tc.authCtx {
				ctx = internalauth.WithClaims(ctx, &internalauth.Claims{Email: tc.email, Name: "Alice"})
			}

			resp, err := h.Me(ctx, connect.NewRequest(&authv1.MeRequest{}))

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

func TestAuthHandler_Callback_InvalidState(t *testing.T) {
	t.Parallel()

	h := NewAuthHandler(nil, nil, nil, nil, nil, config.AuthTypeOIDC)

	req := connect.NewRequest(&authv1.OIDCCallbackRequest{
		State: "valid-state",
		Code:  "auth-code",
	})
	// Provide mismatched state cookie.
	stateCookie := &http.Cookie{Name: oauthStateCookieName, Value: "wrong-state"}
	nonceCookie := &http.Cookie{Name: oauthNonceCookieName, Value: "nonce-val"}
	req.Header().Set("Cookie", stateCookie.String()+"; "+nonceCookie.String())

	_, err := h.OIDCCallback(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestAuthHandler_Callback(t *testing.T) { // NOSONAR
	t.Parallel()

	tests := []struct {
		name         string
		requestState string
		stateCookie  string
		nonceCookie  string
		setupMocks   func(
			provider *auth_mock.MockcallbackProvider,
			users *auth_mock.MockuserUpserter,
		)
		wantErr  bool
		wantCode connect.Code
		// verifyCookies checks that elara_session is set in the response header.
		verifyCookies bool
	}{
		{
			name:          "happy path: valid state cookie sets session cookie",
			requestState:  "test-state",
			stateCookie:   "test-state",
			nonceCookie:   "test-nonce",
			verifyCookies: true,
			setupMocks: func(
				provider *auth_mock.MockcallbackProvider,
				users *auth_mock.MockuserUpserter,
			) {
				provider.EXPECT().
					Exchange(gomock.Any(), "auth-code", "test-nonce").
					Return(&internalauth.Identity{
						Email: "user@example.com",
						Name:  "Test User",
					}, nil)
				users.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name:         "state mismatch returns CodeUnauthenticated",
			requestState: "correct-state",
			stateCookie:  "wrong-state",
			nonceCookie:  "nonce-val",
			wantErr:      true,
			wantCode:     connect.CodeUnauthenticated,
			setupMocks: func(
				_ *auth_mock.MockcallbackProvider,
				_ *auth_mock.MockuserUpserter,
			) {
			},
		},
		{
			name:         "missing state cookie returns CodeUnauthenticated",
			requestState: "some-state",
			stateCookie:  "",
			nonceCookie:  "nonce-val",
			wantErr:      true,
			wantCode:     connect.CodeUnauthenticated,
			setupMocks: func(
				_ *auth_mock.MockcallbackProvider,
				_ *auth_mock.MockuserUpserter,
			) {
			},
		},
		{
			name:         "missing nonce cookie returns CodeUnauthenticated",
			requestState: "test-state",
			stateCookie:  "test-state",
			nonceCookie:  "",
			wantErr:      true,
			wantCode:     connect.CodeUnauthenticated,
			setupMocks: func(
				_ *auth_mock.MockcallbackProvider,
				_ *auth_mock.MockuserUpserter,
			) {
			},
		},
		{
			name:         "callback.Execute exchange error maps to connect error",
			requestState: "test-state",
			stateCookie:  "test-state",
			nonceCookie:  "test-nonce",
			wantErr:      true,
			setupMocks: func(
				provider *auth_mock.MockcallbackProvider,
				_ *auth_mock.MockuserUpserter,
			) {
				provider.EXPECT().
					Exchange(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("provider error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			provider := auth_mock.NewMockcallbackProvider(ctrl)
			users := auth_mock.NewMockuserUpserter(ctrl)

			tt.setupMocks(provider, users)

			session := internalauth.NewSessionManager("test-secret", 0)
			callbackUC := authuc.NewCallbackUseCase(
				provider, users, session,
				nil,
				"",
			)

			h := newTestAuthHandler(nil, callbackUC, nil)

			req := connect.NewRequest(&authv1.OIDCCallbackRequest{
				State: tt.requestState,
				Code:  "auth-code",
			})

			// Build cookie header.
			var cookieParts []string
			if tt.stateCookie != "" {
				cookieParts = append(cookieParts, (&http.Cookie{
					Name:  oauthStateCookieName,
					Value: tt.stateCookie,
				}).String())
			}
			if tt.nonceCookie != "" {
				cookieParts = append(cookieParts, (&http.Cookie{
					Name:  oauthNonceCookieName,
					Value: tt.nonceCookie,
				}).String())
			}
			if len(cookieParts) > 0 {
				req.Header().Set("Cookie", joinCookies(cookieParts))
			}

			resp, err := h.OIDCCallback(t.Context(), req)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantCode != 0 {
					assert.Equal(t, tt.wantCode, connect.CodeOf(err))
				}

				return
			}

			require.NoError(t, err)

			if tt.verifyCookies {
				setCookies := resp.Header().Values(cookieHeader)
				require.NotEmpty(t, setCookies, "expected Set-Cookie header in response")

				found := false
				for _, c := range setCookies {
					if strings.Contains(c, sessionCookieName) {
						found = true

						break
					}
				}
				assert.True(t, found, "elara_session cookie should be set in response")
			}
		})
	}
}

// joinCookies joins multiple cookie strings with "; " for use in the Cookie header.
func joinCookies(parts []string) string {
	return strings.Join(parts, "; ")
}
