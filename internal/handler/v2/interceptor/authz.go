package interceptor

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
)

// AuthzInterceptor handles high-level authorization for ConnectRPC procedures.
type AuthzInterceptor struct {
	authz *authz.Authz
}

var _ connect.Interceptor = (*AuthzInterceptor)(nil)

// NewAuthzInterceptor returns a new AuthzInterceptor.
func NewAuthzInterceptor(az *authz.Authz) *AuthzInterceptor {
	return &AuthzInterceptor{authz: az}
}

func (i *AuthzInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if err := i.authorize(ctx, req.Spec().Procedure); err != nil {
			return nil, err
		}

		return next(ctx, req)
	}
}

func (i *AuthzInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *AuthzInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if err := i.authorize(ctx, conn.Spec().Procedure); err != nil {
			return err
		}

		return next(ctx, conn)
	}
}

func (i *AuthzInterceptor) authorize(ctx context.Context, procedure string) error {
	// 1. All procedures in elara.auth.v1.AuthService are public.
	if strings.HasPrefix(procedure, "/elara.auth.v1.AuthService/") {
		return nil
	}

	// 2. All other procedures require authentication.
	if err := i.authz.RequireAuthenticated(ctx); err != nil {
		return err
	}

	// 3. Procedures in elara.user.v1.UserService, elara.access.v1.GroupService,
	// and elara.access.v1.AccessService require superadmin privileges.
	if strings.HasPrefix(procedure, "/elara.user.v1.UserService/") ||
		strings.HasPrefix(procedure, "/elara.access.v1.GroupService/") ||
		strings.HasPrefix(procedure, "/elara.access.v1.AccessService/") {
		return i.authz.Require(ctx, domain.ObjectAll, domain.ActionAll, domain.DomainAll)
	}

	return nil
}
