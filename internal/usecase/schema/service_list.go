package schema

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

// List returns schemas for the given namespace. Callers without
// (Schema, Read) on the namespace get a silent empty slice — list is a
// filter, not a guarded read.
func (s *Service) List(ctx context.Context, namespace string) ([]*domain.SchemaAttachment, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	if !s.pdp.Has(claims.Email, domain.Permission{
		Object: domain.ObjectSchema,
		Action: domain.ActionRead,
		Domain: namespace,
	}) {
		return nil, nil
	}

	schemas, err := s.store.List(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}

	return schemas, nil
}
