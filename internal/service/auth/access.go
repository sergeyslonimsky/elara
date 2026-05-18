package auth

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_access.go -package=auth_mock -source=access.go

// AccessEnforcer is satisfied by any enforcer that can check a permission.
type AccessEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

// CheckAccess extracts claims from ctx, calls e.Enforce, and returns a typed error.
// Returns ErrUnauthorized if no claims in context, ErrForbidden if not allowed.
func CheckAccess(ctx context.Context, e AccessEnforcer, dom, obj, act string) error {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	allowed, err := e.Enforce(claims.Email, dom, obj, act)
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

	return nil
}

// RequireAuthenticated rejects the call when claims aren't present. Use this
// for handler methods that accept any authenticated user and let the usecase
// decide what each user actually sees (e.g. List endpoints with per-item
// permission filtering).
func RequireAuthenticated(ctx context.Context) error {
	if _, ok := ClaimsFromContext(ctx); !ok {
		return domain.ErrUnauthorized
	}

	return nil
}
