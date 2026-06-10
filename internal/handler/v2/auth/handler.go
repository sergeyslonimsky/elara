package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	sessions_handler "github.com/sergeyslonimsky/elara/internal/handler/v2/sessions"
	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

//go:generate mockgen -destination=mocks/handler_mock.go -package=auth_mock -source=handler.go

const (
	cookieHeader = "Set-Cookie"

	oauthStateCookieName = "elara_oauth_state"
	oauthNonceCookieName = "elara_oauth_nonce"
)

type (
	usecase interface {
		Login(ctx context.Context) (url string, state string, nonce string, err error)
		Callback(ctx context.Context, params authuc.CallbackParams) (*domain.User, *domain.Session, error)
		BasicLogin(ctx context.Context, params authuc.LoginParams) (*domain.User, *domain.Session, error)
	}
)

// Handler implements authv1connect.AuthServiceHandler.
type Handler struct {
	uc           usecase
	authType     domain.AuthType
	secureCookie bool
}

// NewHandler returns a new Handler wired with the login use cases.
func NewHandler(
	uc usecase,
	authType domain.AuthType,
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
	case domain.AuthTypeOIDC:
		authType = authv1.AuthType_AUTH_TYPE_OIDC
	case domain.AuthTypeBasicAuth:
		authType = authv1.AuthType_AUTH_TYPE_BASIC
	case domain.AuthTypeNone:
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
	if h.authType != domain.AuthTypeOIDC {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf(
				"OIDC login is not available: auth type is %s: %w",
				h.authType,
				domain.ErrFeatureNotAvailable,
			),
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
	if h.authType != domain.AuthTypeOIDC {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf(
				"OIDC login is not available: auth type is %s: %w",
				h.authType,
				domain.ErrFeatureNotAvailable,
			),
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

	ip, ua := clientInfo(req.Header())

	user, sess, err := h.uc.Callback(ctx, authuc.CallbackParams{
		Code:      req.Msg.GetCode(),
		Nonce:     nonce,
		IP:        ip,
		UserAgent: ua,
	})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	_ = user // OIDC identity accepted

	resp := connect.NewResponse(&authv1.OIDCCallbackResponse{})
	sessions_handler.SetSessionCookie(resp.Header(), sess.ID, sess.ExpiresAt, h.secureCookie)

	return resp, nil
}

func (h *Handler) BasicLogin(
	ctx context.Context,
	req *connect.Request[authv1.BasicLoginRequest],
) (*connect.Response[authv1.BasicLoginResponse], error) {
	if h.authType != domain.AuthTypeBasicAuth {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf(
				"basic login is not available: auth type is %s: %w",
				h.authType,
				domain.ErrFeatureNotAvailable,
			),
		)
	}

	ip, ua := clientInfo(req.Header())

	user, sess, err := h.uc.BasicLogin(ctx, authuc.LoginParams{
		Email:     req.Msg.GetEmail(),
		Password:  req.Msg.GetPassword(),
		IP:        ip,
		UserAgent: ua,
	})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	resp := connect.NewResponse(&authv1.BasicLoginResponse{
		PasswordChangeRequired: user.PasswordChangeRequired,
	})
	sessions_handler.SetSessionCookie(resp.Header(), sess.ID, sess.ExpiresAt, h.secureCookie)

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

// clientInfo extracts IP and User-Agent from HTTP headers.
// IP is the first value from X-Forwarded-For; User-Agent is taken verbatim.
func clientInfo(header http.Header) (string, string) {
	var ip string

	if xff := header.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		ip = strings.TrimSpace(first)
	}

	return ip, header.Get("User-Agent")
}
