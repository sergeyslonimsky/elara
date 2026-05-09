package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func (s *Service) Delete(ctx context.Context, name string) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	// domain = namespace name itself.
	allowed, err := s.enforcer.Enforce(claims.Email, name, auth.ObjectNamespace, auth.ActionWrite)
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

	count, err := s.store.CountConfigs(ctx, name)
	if err != nil {
		return fmt.Errorf("count configs in namespace: %w", err)
	}

	if count > 0 {
		return domain.NewValidationError("name", fmt.Sprintf("namespace %q contains %d config(s)", name, count))
	}

	if err := s.store.Delete(ctx, name); err != nil {
		return fmt.Errorf("delete namespace: %w", err)
	}

	return nil
}
