package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func (s *Service) Create(ctx context.Context, ns *domain.Namespace) (*domain.Namespace, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(claims.Email, auth.ObjectAll, auth.ObjectNamespace, auth.ActionWrite)
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
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
