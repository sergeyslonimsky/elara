package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Create persists a new namespace. Authorization (namespace/write at
// DomainAll) is enforced by the RBAC interceptor.
func (s *Service) Create(ctx context.Context, ns *domain.Namespace) (*domain.Namespace, error) {
	if err := s.authz.Require(ctx, domain.ObjectNamespace, domain.ActionWrite, domain.DomainAll); err != nil {
		return nil, err
	}

	if err := ns.Validate(); err != nil {
		return nil, fmt.Errorf("validate namespace: %w", err)
	}

	if err := s.store.Create(ctx, ns); err != nil {
		return nil, fmt.Errorf("create namespace: %w", err)
	}

	created, err := s.store.Get(ctx, ns.Name)
	if err != nil {
		return nil, fmt.Errorf("get created namespace: %w", err)
	}

	return created, nil
}
