package v2

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/domain"
	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

const (
	cookieHeader = "Set-Cookie"

	oauthStateCookieName = "elara_oauth_state"
	oauthNonceCookieName = "elara_oauth_nonce"
	sessionCookieName    = "elara_session"
)

// AuthHandler implements authv1connect.AuthServiceHandler.
type AuthHandler struct {
	login          *authuc.LoginUseCase
	callback       *authuc.CallbackUseCase
	me             *authuc.MeUseCase
	basicLogin     *authuc.BasicLoginUseCase
	changePassword *authuc.ChangePasswordUseCase
	authType       config.AuthType
}

// NewAuthHandler returns a new AuthHandler wired with all auth use cases.
func NewAuthHandler(
	login *authuc.LoginUseCase,
	callback *authuc.CallbackUseCase,
	me *authuc.MeUseCase,
	basicLogin *authuc.BasicLoginUseCase,
	changePassword *authuc.ChangePasswordUseCase,
	authType config.AuthType,
) *AuthHandler {
	return &AuthHandler{
		login:          login,
		callback:       callback,
		me:             me,
		basicLogin:     basicLogin,
		changePassword: changePassword,
		authType:       authType,
	}
}

func (h *AuthHandler) GetAuthInfo(
	_ context.Context,
	_ *connect.Request[authv1.GetAuthInfoRequest],
) (*connect.Response[authv1.GetAuthInfoResponse], error) {
	var authType authv1.AuthType
	switch h.authType {
	case config.AuthTypeOIDC:
		authType = authv1.AuthType_AUTH_TYPE_OIDC
	case config.AuthTypeBasicAuth:
		authType = authv1.AuthType_AUTH_TYPE_BASIC
	case config.AuthTypeNone:
		authType = authv1.AuthType_AUTH_TYPE_NONE
	default:
		authType = authv1.AuthType_AUTH_TYPE_UNSPECIFIED
	}

	return connect.NewResponse(&authv1.GetAuthInfoResponse{
		AuthType: authType,
	}), nil
}

func (h *AuthHandler) OIDCLogin(
	ctx context.Context,
	_ *connect.Request[authv1.OIDCLoginRequest],
) (*connect.Response[authv1.OIDCLoginResponse], error) {
	if h.authType != config.AuthTypeOIDC {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("OIDC login is not available: auth type is %s: %w", h.authType, domain.ErrFeatureNotAvailable),
		)
	}

	redirectURL, state, nonce, err := h.login.Execute(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}

	resp := connect.NewResponse(&authv1.OIDCLoginResponse{
		RedirectUrl: redirectURL,
	})

	stateCookie := &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/auth",
	}
	resp.Header().Add(cookieHeader, stateCookie.String())

	nonceCookie := &http.Cookie{
		Name:     oauthNonceCookieName,
		Value:    nonce,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/auth",
	}
	resp.Header().Add(cookieHeader, nonceCookie.String())

	return resp, nil
}

func (h *AuthHandler) OIDCCallback(
	ctx context.Context,
	req *connect.Request[authv1.OIDCCallbackRequest],
) (*connect.Response[authv1.OIDCCallbackResponse], error) {
	if h.authType != config.AuthTypeOIDC {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("OIDC login is not available: auth type is %s: %w", h.authType, domain.ErrFeatureNotAvailable),
		)
	}

	expectedState, err := extractCookieFromRequest(req.Header(), oauthStateCookieName)
	if err != nil || expectedState != req.Msg.GetState() {
		return nil, connect.NewError(connect.CodeUnauthenticated, domain.ErrUnauthorized)
	}

	nonce, err := extractCookieFromRequest(req.Header(), oauthNonceCookieName)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, domain.ErrUnauthorized)
	}

	sessionToken, _, err := h.callback.Execute(ctx, req.Msg.GetCode(), nonce)
	if err != nil {
		return nil, toConnectError(err)
	}

	resp := connect.NewResponse(&authv1.OIDCCallbackResponse{})

	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionToken,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	}
	resp.Header().Add(cookieHeader, cookie.String())

	return resp, nil
}

func (h *AuthHandler) BasicLogin(
	ctx context.Context,
	req *connect.Request[authv1.BasicLoginRequest],
) (*connect.Response[authv1.BasicLoginResponse], error) {
	if h.authType != config.AuthTypeBasicAuth {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("basic login is not available: auth type is %s: %w", h.authType, domain.ErrFeatureNotAvailable),
		)
	}

	sessionToken, user, err := h.basicLogin.Execute(ctx, req.Msg.GetEmail(), req.Msg.GetPassword())
	if err != nil {
		return nil, toConnectError(err)
	}

	resp := connect.NewResponse(&authv1.BasicLoginResponse{
		PasswordChangeRequired: user.PasswordChangeRequired,
	})

	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionToken,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	}
	resp.Header().Add(cookieHeader, cookie.String())

	return resp, nil
}

func (h *AuthHandler) ChangePassword(
	ctx context.Context,
	req *connect.Request[authv1.ChangePasswordRequest],
) (*connect.Response[authv1.ChangePasswordResponse], error) {
	if h.authType != config.AuthTypeBasicAuth {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf(
				"change password is not available: auth type is %s: %w",
				h.authType,
				domain.ErrFeatureNotAvailable,
			),
		)
	}

	err := h.changePassword.Execute(ctx, req.Msg.GetCurrentPassword(), req.Msg.GetNewPassword())
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.ChangePasswordResponse{}), nil
}

func (h *AuthHandler) Logout(
	_ context.Context,
	_ *connect.Request[authv1.LogoutRequest],
) (*connect.Response[authv1.LogoutResponse], error) {
	resp := connect.NewResponse(&authv1.LogoutResponse{})

	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		MaxAge:   -1,
	}
	resp.Header().Add(cookieHeader, cookie.String())

	return resp, nil
}

func (h *AuthHandler) Me(
	ctx context.Context,
	_ *connect.Request[authv1.MeRequest],
) (*connect.Response[authv1.MeResponse], error) {
	result, err := h.me.Execute(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}

	namespaces := make([]*authv1.NamespaceAccess, 0, len(result.Namespaces))
	for _, ns := range result.Namespaces {
		namespaces = append(namespaces, &authv1.NamespaceAccess{
			Name:     ns.Name,
			CanWrite: ns.CanWrite,
		})
	}

	claims, _ := auth.ClaimsFromContext(ctx)

	return connect.NewResponse(&authv1.MeResponse{
		Email:                  result.Email,
		Name:                   result.Name,
		IsAdmin:                result.IsAdmin,
		Namespaces:             namespaces,
		CanViewWebhooks:        result.CanViewWebhooks,
		CanManageWebhooks:      result.CanManageWebhooks,
		PasswordChangeRequired: claims.PasswordChangeRequired,
	}), nil
}

func extractCookieFromRequest(header http.Header, name string) (string, error) {
	req := &http.Request{Header: header}

	cookie, err := req.Cookie(name)
	if err != nil {
		return "", fmt.Errorf("read cookie %q: %w", name, err)
	}

	return cookie.Value, nil
}
