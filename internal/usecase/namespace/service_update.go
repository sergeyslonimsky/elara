package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Update changes a namespace's description. Authorization
// (namespace/write on `name`) is enforced at the handler boundary.
func (s *Service) Update(ctx context.Context, name, description string) (*domain.Namespace, error) {
	ns := &domain.Namespace{
		Name:        name,
		Description: description,
	}

	var updated *domain.Namespace

	err := s.txm.WithTx(ctx, func(ctx context.Context) error {
		if err := s.store.Update(ctx, ns); err != nil {
			return fmt.Errorf("update namespace: %w", err)
		}

		u, err := s.store.Get(ctx, name)
		if err != nil {
			return fmt.Errorf("get updated namespace: %w", err)
		}
		updated = u

		return s.populateConfigCount(ctx, updated)
	})
	if err != nil {
		return nil, fmt.Errorf("update namespace tx: %w", err)
	}

	return updated, nil
}
