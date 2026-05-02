package v2

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"

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
	login    *authuc.LoginUseCase
	callback *authuc.CallbackUseCase
	me       *authuc.MeUseCase
}

// NewAuthHandler returns a new AuthHandler wired with all auth use cases.
func NewAuthHandler(
	login *authuc.LoginUseCase,
	callback *authuc.CallbackUseCase,
	me *authuc.MeUseCase,
) *AuthHandler {
	return &AuthHandler{
		login:    login,
		callback: callback,
		me:       me,
	}
}

func (h *AuthHandler) Login(
	ctx context.Context,
	_ *connect.Request[authv1.LoginRequest],
) (*connect.Response[authv1.LoginResponse], error) {
	redirectURL, state, nonce, err := h.login.Execute(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}

	resp := connect.NewResponse(&authv1.LoginResponse{
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

func (h *AuthHandler) Callback(
	ctx context.Context,
	req *connect.Request[authv1.CallbackRequest],
) (*connect.Response[authv1.CallbackResponse], error) {
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

	resp := connect.NewResponse(&authv1.CallbackResponse{})

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
	user, roles, err := h.me.Execute(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.MeResponse{
		Email:   user.Email,
		Name:    user.Name,
		Picture: user.Picture,
		Roles:   roles,
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
