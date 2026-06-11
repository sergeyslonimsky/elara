package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

// Stable, user-facing OIDC callback errors. Returned through ConnectRPC so the
// SPA renders a fixed message instead of leaking raw oauth2 / casbin / bbolt
// internals. The original wrapped error is logged for operators in
// mapOIDCCallbackError. These intentionally violate the ST1005 convention
// (lowercase, no punctuation) because they are rendered verbatim in the UI.
//
//nolint:staticcheck // ST1005: user-facing UI strings, sentence-cased on purpose
var (
	errAccountNotProvisioned = errors.New("Account not provisioned. Contact your administrator.")
	errAccountDeactivated    = errors.New("Account deactivated. Contact your administrator.")
	errAuthorizationExpired  = errors.New("Authorization expired. Please log in again.")
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
	sessions_handler.SetOAuthStateCookie(resp.Header(), state, h.secureCookie)
	sessions_handler.SetOAuthNonceCookie(resp.Header(), nonce, h.secureCookie)

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

	expectedState, err := extractCookieFromRequest(req.Header(), sessions_handler.OAuthStateCookieName)
	if err != nil || expectedState != req.Msg.GetState() {
		return nil, connect.NewError(connect.CodeUnauthenticated, domain.ErrUnauthorized)
	}

	nonce, err := extractCookieFromRequest(req.Header(), sessions_handler.OAuthNonceCookieName)
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
		return nil, mapOIDCCallbackError(ctx, err)
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

// mapOIDCCallbackError converts a usecase-level Callback error into a
// ConnectRPC error with a stable, user-facing message. The original wrapped
// error is logged for operators; the message returned to the SPA is a clean
// constant string keyed on error class so the UI can react deterministically
// (and so we never leak raw oauth2 / bbolt / casbin internals to the browser).
func mapOIDCCallbackError(ctx context.Context, err error) *connect.Error {
	slog.ErrorContext(ctx, "oidc callback failed", "error", err)

	switch {
	case errors.Is(err, domain.ErrIdentityNotProvisioned):
		return connect.NewError(connect.CodePermissionDenied, errAccountNotProvisioned)
	case errors.Is(err, domain.ErrUserDeactivated):
		return connect.NewError(connect.CodePermissionDenied, errAccountDeactivated)
	case strings.Contains(err.Error(), "invalid_grant"):
		// oauth2 code reuse / expired authorization. Either a double-fire on
		// the client or a stale tab — SPA should redirect back to /login.
		return connect.NewError(connect.CodeFailedPrecondition, errAuthorizationExpired)
	default:
		return v2.ToConnectError(err)
	}
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
