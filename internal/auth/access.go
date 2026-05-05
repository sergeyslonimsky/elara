package auth

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

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
