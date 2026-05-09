package schema

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func (s *Service) Detach(ctx context.Context, namespace, pathPattern string) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(claims.Email, namespace, "schema", "write")
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

	ns, err := s.namespaces.Get(ctx, namespace)
	if err != nil {
		return fmt.Errorf("get namespace: %w", err)
	}

	if ns.Locked {
		return fmt.Errorf("namespace %q: %w", namespace, domain.ErrNamespaceLocked)
	}

	if err := s.store.Detach(ctx, namespace, pathPattern); err != nil {
		return fmt.Errorf("detach schema: %w", err)
	}

	return nil
}
