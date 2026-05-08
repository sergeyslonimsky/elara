package profile

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	profilev1 "github.com/sergeyslonimsky/elara/internal/proto/elara/profile/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

const (
	cookieHeader      = "Set-Cookie"
	sessionCookieName = "elara_session"
)

// Handler implements profilev1connect.ProfileServiceHandler.
type Handler struct {
	me             *authuc.MeUseCase
	changePassword *authuc.ChangePasswordUseCase
	authType       config.AuthType
	secureCookie   bool
}

// New returns a new Handler wired with profile use cases.
func New(
	me *authuc.MeUseCase,
	changePassword *authuc.ChangePasswordUseCase,
	authType config.AuthType,
	secureCookie bool,
) *Handler {
	return &Handler{
		me:             me,
		changePassword: changePassword,
		authType:       authType,
		secureCookie:   secureCookie,
	}
}

func (h *Handler) Me(
	ctx context.Context,
	_ *connect.Request[profilev1.MeRequest],
) (*connect.Response[profilev1.MeResponse], error) {
	result, err := h.me.Execute(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	namespaces := make([]*profilev1.NamespaceAccess, 0, len(result.Namespaces))
	for _, ns := range result.Namespaces {
		namespaces = append(namespaces, &profilev1.NamespaceAccess{
			Name:     ns.Name,
			CanWrite: ns.CanWrite,
		})
	}

	claims, _ := auth.ClaimsFromContext(ctx)

	return connect.NewResponse(&profilev1.MeResponse{
		Email:                  result.Email,
		Name:                   result.Name,
		IsAdmin:                result.IsAdmin,
		Namespaces:             namespaces,
		CanViewWebhooks:        result.CanViewWebhooks,
		CanManageWebhooks:      result.CanManageWebhooks,
		PasswordChangeRequired: claims.PasswordChangeRequired,
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

	token, err := h.changePassword.Execute(ctx, req.Msg.GetCurrentPassword(), req.Msg.GetNewPassword())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	resp := connect.NewResponse(&profilev1.ChangePasswordResponse{})
	cookie := &http.Cookie{ //nolint:gosec //Secure set from config
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
	_ context.Context,
	_ *connect.Request[profilev1.LogoutRequest],
) (*connect.Response[profilev1.LogoutResponse], error) {
	resp := connect.NewResponse(&profilev1.LogoutResponse{})

	cookie := &http.Cookie{ //nolint:gosec //Secure set from config
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
