package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Create persists a new namespace. Authorization is enforced at the handler
// boundary (namespace/create on DomainAll).
func (s *Service) Create(ctx context.Context, ns *domain.Namespace) (*domain.Namespace, error) {
	if err := ns.Validate(); err != nil {
		return nil, fmt.Errorf("validate namespace: %w", err)
	}

	var created *domain.Namespace

	err := s.txm.WithTx(ctx, func(ctx context.Context) error {
		if err := s.store.Create(ctx, ns); err != nil {
			return fmt.Errorf("create namespace: %w", err)
		}

		u, err := s.store.Get(ctx, ns.Name)
		if err != nil {
			return fmt.Errorf("get created namespace: %w", err)
		}
		created = u

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create namespace tx: %w", err)
	}

	return created, nil
}
