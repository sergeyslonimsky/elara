package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Delete removes a namespace. Authorization is enforced at the handler
// boundary (admin-only via DomainAll namespace/write).
func (s *Service) Delete(ctx context.Context, name string) error {
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
}
