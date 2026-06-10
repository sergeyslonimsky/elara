package namespace

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

// Update changes a namespace's description. Authorization
// (namespace/write on `name`) is enforced at the handler boundary.
func (s *Service) Update(ctx context.Context, name, description string) (*domain.Namespace, error) {
	var updated *domain.Namespace

	err := s.txm.WithTx(ctx, func(ctx context.Context) error {
		existing, err := s.store.Get(ctx, name)
		if err != nil {
			if errors.Is(err, storage.ErrResourceNotFound) {
				return fmt.Errorf("get namespace: %w", domain.ErrNotFound)
			}

			return fmt.Errorf("get namespace: %w", err)
		}

		if existing.Locked {
			return fmt.Errorf("namespace %q: %w", name, domain.ErrNamespaceLocked)
		}

		existing.Description = description
		existing.UpdatedAt = time.Now()

		if err := s.store.Update(ctx, existing); err != nil {
			return fmt.Errorf("update namespace: %w", err)
		}

		updated = existing

		return s.populateConfigCount(ctx, updated)
	})
	if err != nil {
		return nil, fmt.Errorf("update namespace tx: %w", err)
	}

	return updated, nil
}
