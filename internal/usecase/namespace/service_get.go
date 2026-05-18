package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Get returns a single namespace. Authorization (namespace/read on `name`)
// is enforced at the handler boundary.
func (s *Service) Get(ctx context.Context, name string) (*domain.Namespace, error) {
	if err := s.authz.Require(ctx, domain.ObjectNamespace, domain.ActionRead, name); err != nil {
		return nil, err
	}

	ns, err := s.store.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get namespace: %w", err)
	}

	if err := s.populateConfigCount(ctx, ns); err != nil {
		return nil, err
	}

	return ns, nil
}
