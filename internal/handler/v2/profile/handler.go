package profile

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/permission"
	sessions_handler "github.com/sergeyslonimsky/elara/internal/handler/v2/sessions"
	profilev1 "github.com/sergeyslonimsky/elara/internal/proto/elara/profile/v1"
	profileuc "github.com/sergeyslonimsky/elara/internal/usecase/profile"
)

//go:generate mockgen -destination=mocks/handler_mock.go -package=profile_mock -source=handler.go

type (
	usecase interface {
		Me(ctx context.Context) (*profileuc.MeResult, error)
		ChangePassword(ctx context.Context, params profileuc.ChangePasswordParams) (*domain.Session, error)
		Logout(ctx context.Context, sessionID, revokedBy string) error
	}
)

// Handler implements profilev1connect.ProfileServiceHandler.
type Handler struct {
	uc           usecase
	authType     domain.AuthType
	secureCookie bool
}

// New returns a new Handler wired with profile use cases.
func New(
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

func (h *Handler) Me(
	ctx context.Context,
	_ *connect.Request[profilev1.MeRequest],
) (*connect.Response[profilev1.MeResponse], error) {
	result, err := h.uc.Me(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	user, _ := authctx.UserFromContext(ctx)

	var passwordChangeRequired bool
	if user != nil {
		passwordChangeRequired = user.PasswordChangeRequired
	}

	return connect.NewResponse(&profilev1.MeResponse{
		Email:                  result.Email,
		Name:                   result.Name,
		PasswordChangeRequired: passwordChangeRequired,
		Permissions:            permission.AssignmentsToProto(result.Permissions),
	}), nil
}

func (h *Handler) ChangePassword(
	ctx context.Context,
	req *connect.Request[profilev1.ChangePasswordRequest],
) (*connect.Response[profilev1.ChangePasswordResponse], error) {
	if h.authType != domain.AuthTypeBasicAuth {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf(
				"change password is not available: auth type is %s: %w",
				h.authType,
				domain.ErrFeatureNotAvailable,
			),
		)
	}

	ip, ua := extractClientInfo(req.Header())

	sess, err := h.uc.ChangePassword(ctx, profileuc.ChangePasswordParams{
		CurrentPassword: req.Msg.GetCurrentPassword(),
		NewPassword:     req.Msg.GetNewPassword(),
		IP:              ip,
		UserAgent:       ua,
	})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	resp := connect.NewResponse(&profilev1.ChangePasswordResponse{})
	sessions_handler.SetSessionCookie(resp.Header(), sess.ID, sess.ExpiresAt, h.secureCookie)

	return resp, nil
}

func (h *Handler) Logout(
	ctx context.Context,
	req *connect.Request[profilev1.LogoutRequest],
) (*connect.Response[profilev1.LogoutResponse], error) {
	sess, ok := authctx.SessionFromContext(ctx)
	if ok {
		user, _ := authctx.UserFromContext(ctx)

		revokedBy := ""
		if user != nil {
			revokedBy = user.Email
		}

		// Logout MUST NOT fail the client response — clearing the cookie
		// below is the user-visible side of "I logged out" — but any
		// server-side revoke failure (bbolt write fail, audit append
		// fail) leaves the session row alive after the cookie is gone.
		// Surface those failures to operators via slog so the discrepancy
		// is observable instead of silently dropped.
		if err := h.uc.Logout(ctx, sess.ID, revokedBy); err != nil {
			slog.WarnContext(ctx, "logout revoke failed", "session_id", sess.ID, "err", err)
		}
	}

	resp := connect.NewResponse(&profilev1.LogoutResponse{})
	sessions_handler.ClearSessionCookie(resp.Header(), h.secureCookie)

	return resp, nil
}

// extractClientInfo extracts IP and User-Agent from HTTP headers.
// IP is the first value from X-Forwarded-For; User-Agent is taken as-is.
func extractClientInfo(header http.Header) (string, string) {
	var ip string

	if xff := header.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		ip = strings.TrimSpace(first)
	}

	return ip, header.Get("User-Agent")
}
