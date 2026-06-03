package namespace

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

// Get returns a single namespace. Authorization (namespace/read on `name`)
// is enforced at the handler boundary.
func (s *Service) Get(ctx context.Context, name string) (*domain.Namespace, error) {
	ns, err := s.store.Get(ctx, name)
	if err != nil {
		if errors.Is(err, storage.ErrResourceNotFound) {
			return nil, fmt.Errorf("get namespace: %w", domain.ErrNotFound)
		}

		return nil, fmt.Errorf("get namespace: %w", err)
	}

	if err := s.populateConfigCount(ctx, ns); err != nil {
		return nil, err
	}

	return ns, nil
}
