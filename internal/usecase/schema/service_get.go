package schema

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func (s *Service) Get(ctx context.Context, namespace, pathPattern string) (*domain.SchemaAttachment, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(claims.Email, namespace, "schema", "read")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	attachment, err := s.store.Get(ctx, namespace, pathPattern)
	if err != nil {
		return nil, fmt.Errorf("get schema: %w", err)
	}

	return attachment, nil
}

func (s *Service) GetEffective(ctx context.Context, namespace, path string) (*domain.SchemaAttachment, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(claims.Email, namespace, "schema", "read")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	schemas, err := s.store.List(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}

	return findBestMatch(schemas, path), nil
}
