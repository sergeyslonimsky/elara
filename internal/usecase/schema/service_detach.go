package schema

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func (s *Service) Detach(ctx context.Context, namespace, pathPattern string) error {
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
