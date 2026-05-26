package profile

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/permission"
	profilev1 "github.com/sergeyslonimsky/elara/internal/proto/elara/profile/v1"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	profileuc "github.com/sergeyslonimsky/elara/internal/usecase/profile"
)

//go:generate mockgen -destination=mocks/handler_mock.go -package=profile_mock -source=handler.go

const (
	cookieHeader      = "Set-Cookie"
	sessionCookieName = "elara_session"
)

type usecase interface {
	Me(ctx context.Context) (*profileuc.MeResult, error)
	ChangePassword(ctx context.Context, currentPassword, newPassword string) (string, error)
	Logout(_ context.Context) error
}

// Handler implements profilev1connect.ProfileServiceHandler.
type Handler struct {
	uc           usecase
	authType     config.AuthType
	secureCookie bool
}

// New returns a new Handler wired with profile use cases.
func New(
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

func (h *Handler) Me(
	ctx context.Context,
	_ *connect.Request[profilev1.MeRequest],
) (*connect.Response[profilev1.MeResponse], error) {
	result, err := h.uc.Me(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	claims, _ := auth.ClaimsFromContext(ctx)

	return connect.NewResponse(&profilev1.MeResponse{
		Email:                  result.Email,
		Name:                   result.Name,
		PasswordChangeRequired: claims.PasswordChangeRequired,
		Permissions:            permission.AssignmentsToProto(result.Permissions),
	}), nil
}

func (h *Handler) ChangePassword(
	ctx context.Context,
	req *connect.Request[profilev1.ChangePasswordRequest],
) (*connect.Response[profilev1.ChangePasswordResponse], error) {
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

	token, err := h.uc.ChangePassword(ctx, req.Msg.GetCurrentPassword(), req.Msg.GetNewPassword())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	resp := connect.NewResponse(&profilev1.ChangePasswordResponse{})
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	}
	resp.Header().Add(cookieHeader, cookie.String())

	return resp, nil
}

func (h *Handler) Logout(
	ctx context.Context,
	_ *connect.Request[profilev1.LogoutRequest],
) (*connect.Response[profilev1.LogoutResponse], error) {
	_ = h.uc.Logout(ctx)
	resp := connect.NewResponse(&profilev1.LogoutResponse{})

	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		MaxAge:   -1,
	}
	resp.Header().Add(cookieHeader, cookie.String())

	return resp, nil
}
