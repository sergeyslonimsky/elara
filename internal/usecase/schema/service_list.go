package schema

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func (s *Service) List(ctx context.Context, namespace string) ([]*domain.SchemaAttachment, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	// Filter silently by namespace.
	allowed, _ := s.enforcer.Enforce(claims.Email, namespace, "schema", "read")
	if !allowed {
		return nil, nil
	}

	schemas, err := s.store.List(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}

	return schemas, nil
}
