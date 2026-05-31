package authctx

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type (
	claimsKey  struct{}
	userKey    struct{}
	sessionKey struct{}
)

// WithClaims stores JWT claims in context. Retained for backward compatibility
// during the EL-49 session migration; prefer WithSession/UserFromContext for new code.
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, claims)
}

// ClaimsFromContext returns claims stored by WithClaims. Used by the etcd v3
// token interceptor (service-credential auth path). v2 handlers should call
// UserFromContext / AuthInfoFromContext instead.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsKey{}).(*Claims)

	return claims, ok
}

// WithSession stores the authenticated domain.Session and domain.User in context.
// Called by the session-based AuthInterceptor (EL-49).
func WithSession(ctx context.Context, sess *domain.Session, user *domain.User) context.Context {
	ctx = context.WithValue(ctx, sessionKey{}, sess)
	ctx = context.WithValue(ctx, userKey{}, user)

	return ctx
}

// SessionFromContext returns the *domain.Session injected by the session-based interceptor.
func SessionFromContext(ctx context.Context) (*domain.Session, bool) {
	sess, ok := ctx.Value(sessionKey{}).(*domain.Session)

	return sess, ok
}

// UserFromContext returns the *domain.User injected by the session-based interceptor.
func UserFromContext(ctx context.Context) (*domain.User, bool) {
	user, ok := ctx.Value(userKey{}).(*domain.User)

	return user, ok
}

// AuthInfoFromContext resolves domain.AuthInfo from context.
// It checks the new session-based user first, then falls back to JWT claims.
func AuthInfoFromContext(ctx context.Context) (domain.AuthInfo, error) {
	if user, ok := UserFromContext(ctx); ok {
		return domain.AuthInfo{
			Email: user.Email,
			Name:  user.Name,
		}, nil
	}

	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return domain.AuthInfo{}, domain.ErrUnauthorized
	}

	return domain.AuthInfo{
		Email:      claims.Email,
		Name:       claims.Name,
		Namespaces: claims.Namespaces,
		Role:       claims.Role,
	}, nil
}
