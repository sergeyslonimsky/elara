package namespace

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

// Delete removes a namespace. Authorization is enforced at the handler
// boundary (admin-only via DomainAll namespace/write).
func (s *Service) Delete(ctx context.Context, name string) error {
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

		count, err := s.store.CountConfigs(ctx, name)
		if err != nil {
			return fmt.Errorf("count configs in namespace: %w", err)
		}

		if count > 0 {
			return domain.NewValidationError(
				"name",
				fmt.Sprintf("namespace %q contains %d config(s)", name, count),
			)
		}

		if err := s.store.Delete(ctx, name); err != nil {
			return fmt.Errorf("delete namespace: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("delete namespace tx: %w", err)
	}

	return nil
}
