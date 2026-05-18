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

	if err := s.store.Update(ctx, ns); err != nil {
		return nil, fmt.Errorf("update namespace: %w", err)
	}

	updated, err := s.store.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get updated namespace: %w", err)
	}

	if err := s.populateConfigCount(ctx, updated); err != nil {
		return nil, err
	}

	return updated, nil
}
