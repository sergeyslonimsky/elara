package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func (s *Service) Get(ctx context.Context, name string) (*domain.Namespace, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	// domain = namespace name itself.
	allowed, err := s.enforcer.Enforce(claims.Email, name, auth.ObjectNamespace, auth.ActionRead)
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
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
