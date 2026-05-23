package auth

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type contextKey struct{}

func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, contextKey{}, claims)
}

func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(contextKey{}).(*Claims)

	return claims, ok
}

func AuthInfoFromContext(ctx context.Context) (domain.AuthInfo, error) {
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
