package auth

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
)

//go:generate mockgen -destination=mocks/handler_mock.go -package=auth_mock -source=handler.go

const (
	cookieHeader = "Set-Cookie"

	oauthStateCookieName = "elara_oauth_state"
	oauthNonceCookieName = "elara_oauth_nonce"
	sessionCookieName    = "elara_session"
)

type usecase interface {
	Login(ctx context.Context) (url string, state string, nonce string, err error)
	Callback(ctx context.Context, code, nonce string) (string, *domain.User, error)
	BasicLogin(ctx context.Context, email, password string) (string, *domain.User, error)
}

// Handler implements authv1connect.AuthServiceHandler.
type Handler struct {
	uc           usecase
	authType     config.AuthType
	secureCookie bool
}

// NewHandler returns a new Handler wired with the login use cases.
func NewHandler(
	uc usecase,
	authType config.AuthType,
	secureCookie bool,
) *Handler {
	return &Handler{
		uc:           uc,
		authType:     authType,
		secureCookie: secureCookie,
	}
}

func (h *Handler) GetAuthInfo(
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

func (h *Handler) OIDCLogin(
	ctx context.Context,
	_ *connect.Request[authv1.OIDCLoginRequest],
) (*connect.Response[authv1.OIDCLoginResponse], error) {
	if h.authType != config.AuthTypeOIDC {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("OIDC login is not available: auth type is %s: %w", h.authType, domain.ErrFeatureNotAvailable),
		)
	}

	redirectURL, state, nonce, err := h.uc.Login(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
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

func (h *Handler) OIDCCallback(
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

	sessionToken, _, err := h.uc.Callback(ctx, req.Msg.GetCode(), nonce)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	resp := connect.NewResponse(&authv1.OIDCCallbackResponse{})

	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionToken,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	}
	resp.Header().Add(cookieHeader, cookie.String())

	return resp, nil
}

func (h *Handler) BasicLogin(
	ctx context.Context,
	req *connect.Request[authv1.BasicLoginRequest],
) (*connect.Response[authv1.BasicLoginResponse], error) {
	if h.authType != config.AuthTypeBasicAuth {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("basic login is not available: auth type is %s: %w", h.authType, domain.ErrFeatureNotAvailable),
		)
	}

	sessionToken, user, err := h.uc.BasicLogin(ctx, req.Msg.GetEmail(), req.Msg.GetPassword())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	resp := connect.NewResponse(&authv1.BasicLoginResponse{
		PasswordChangeRequired: user.PasswordChangeRequired,
	})

	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionToken,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	}
	resp.Header().Add(cookieHeader, cookie.String())

	return resp, nil
}

func extractCookieFromRequest(header http.Header, name string) (string, error) {
	req := &http.Request{Header: header}

	cookie, err := req.Cookie(name)
	if err != nil {
		return "", fmt.Errorf("read cookie %q: %w", name, err)
	}

	return cookie.Value, nil
}
