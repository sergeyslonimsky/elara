package interceptor

import (
	"context"
	"net/http"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

const sessionCookieName = "elara_session"

//nolint:gochecknoglobals // explicitly requested by code review CR-5 to optimize allocations
var passwordChangeAllowedProcedures = map[string]struct{}{
	"/elara.profile.v1.ProfileService/ChangePassword": {},
	"/elara.profile.v1.ProfileService/Logout":         {},
	"/elara.profile.v1.ProfileService/Me":             {},
}

// AuthInterceptor validates the elara_session cookie and injects *auth.Claims into context.
type AuthInterceptor struct {
	session *auth.SessionManager
}

var _ connect.Interceptor = (*AuthInterceptor)(nil)

// NewAuthInterceptor returns an AuthInterceptor that authenticates all requests.
func NewAuthInterceptor(session *auth.SessionManager) *AuthInterceptor {
	return &AuthInterceptor{session: session}
}

func (i *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := i.authenticate(ctx, req.Header())
		if err != nil {
			return nil, err
		}

		if err := i.checkPasswordChangeRequired(ctx, req.Spec().Procedure); err != nil {
			return nil, err
		}

		return next(ctx, req)
	}
}

func (i *AuthInterceptor) WrapStreamingClient(
	next connect.StreamingClientFunc,
) connect.StreamingClientFunc {
	return next
}

func (i *AuthInterceptor) WrapStreamingHandler(
	next connect.StreamingHandlerFunc,
) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.authenticate(ctx, conn.RequestHeader())
		if err != nil {
			return err
		}

		if err := i.checkPasswordChangeRequired(ctx, conn.Spec().Procedure); err != nil {
			return err
		}

		return next(ctx, conn)
	}
}

func (i *AuthInterceptor) authenticate(
	ctx context.Context,
	header http.Header,
) (context.Context, error) {
	cookieValue, err := extractCookie(header, sessionCookieName)
	if err != nil {
		return ctx, connect.NewError(connect.CodeUnauthenticated, domain.ErrUnauthorized)
	}

	claims, err := i.session.Validate(cookieValue)
	if err != nil {
		return ctx, connect.NewError(connect.CodeUnauthenticated, domain.ErrUnauthorized)
	}

	return auth.WithClaims(ctx, claims), nil
}

func (i *AuthInterceptor) checkPasswordChangeRequired(ctx context.Context, procedure string) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil
	}

	if claims.PasswordChangeRequired {
		if _, ok := passwordChangeAllowedProcedures[procedure]; !ok {
			return connect.NewError(connect.CodePermissionDenied, domain.ErrPasswordChangeRequired)
		}
	}

	return nil
}

//nolint:wrapcheck // caller converts this to a connect error; wrapping the stdlib http error adds no value
func extractCookie(header http.Header, name string) (string, error) {
	req := &http.Request{Header: header}
	cookie, err := req.Cookie(name)
	if err != nil {
		return "", err
	}

	return cookie.Value, nil
}
