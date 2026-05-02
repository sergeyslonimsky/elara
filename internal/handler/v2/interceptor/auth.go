package interceptor

import (
	"context"
	"net/http"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

const sessionCookieName = "elara_session"

// AuthInterceptor validates the elara_session cookie and injects *auth.Claims into context.
// Procedures listed in publicProc bypass the auth check entirely.
type AuthInterceptor struct {
	session    *auth.SessionManager
	publicProc map[string]struct{}
}

var _ connect.Interceptor = (*AuthInterceptor)(nil)

// NewAuthInterceptor returns an AuthInterceptor that skips auth for the listed public procedures.
func NewAuthInterceptor(session *auth.SessionManager, publicProcedures []string) *AuthInterceptor {
	m := make(map[string]struct{}, len(publicProcedures))
	for _, p := range publicProcedures {
		m[p] = struct{}{}
	}

	return &AuthInterceptor{session: session, publicProc: m}
}

func (i *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if _, ok := i.publicProc[req.Spec().Procedure]; ok {
			return next(ctx, req)
		}

		ctx, err := i.authenticate(ctx, req.Header())
		if err != nil {
			return nil, err
		}

		return next(ctx, req)
	}
}

func (i *AuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *AuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if _, ok := i.publicProc[conn.Spec().Procedure]; ok {
			return next(ctx, conn)
		}

		ctx, err := i.authenticate(ctx, conn.RequestHeader())
		if err != nil {
			return err
		}

		return next(ctx, conn)
	}
}

func (i *AuthInterceptor) authenticate(ctx context.Context, header http.Header) (context.Context, error) {
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

//nolint:wrapcheck // caller converts this to a connect error; wrapping the stdlib http error adds no value
func extractCookie(header http.Header, name string) (string, error) {
	req := &http.Request{Header: header}
	cookie, err := req.Cookie(name)
	if err != nil {
		return "", err
	}

	return cookie.Value, nil
}
