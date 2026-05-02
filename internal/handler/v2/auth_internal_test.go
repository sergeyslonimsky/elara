package v2

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	internalauth "github.com/sergeyslonimsky/elara/internal/auth"
	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

func newTestAuthHandler(
	loginUC *authuc.LoginUseCase,
	callbackUC *authuc.CallbackUseCase,
	meUC *authuc.MeUseCase,
) *AuthHandler {
	return NewAuthHandler(loginUC, callbackUC, meUC)
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

			resp, err := h.Login(context.Background(), connect.NewRequest(&authv1.LoginRequest{}))

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

func TestAuthHandler_Logout(t *testing.T) {
	t.Parallel()

	h := NewAuthHandler(nil, nil, nil)

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
		roles    []string
		roleErr  error
		wantErr  bool
		wantCode connect.Code
	}{
		{
			name:    "returns user and roles",
			email:   "alice@example.com",
			authCtx: true,
			roles:   []string{"role:admin"},
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

			ctrl := gomock.NewController(t)
			roleGetter := auth_mock.NewMockroleGetter(ctrl)

			if tc.authCtx {
				roleGetter.EXPECT().GetRolesForUser(tc.email, "*").Return(tc.roles, tc.roleErr)
			}

			meUC := authuc.NewMeUseCase(roleGetter)
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
			assert.Equal(t, tc.roles, resp.Msg.GetRoles())
		})
	}
}

func TestAuthHandler_Callback_InvalidState(t *testing.T) {
	t.Parallel()

	h := NewAuthHandler(nil, nil, nil)

	req := connect.NewRequest(&authv1.CallbackRequest{
		State: "valid-state",
		Code:  "auth-code",
	})
	// Provide mismatched state cookie.
	stateCookie := &http.Cookie{Name: oauthStateCookieName, Value: "wrong-state"}
	nonceCookie := &http.Cookie{Name: oauthNonceCookieName, Value: "nonce-val"}
	req.Header().Set("Cookie", stateCookie.String()+"; "+nonceCookie.String())

	_, err := h.Callback(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
