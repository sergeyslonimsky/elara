package schema

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

// List returns schemas for the given namespace. Callers without
// (Schema, Read) on the namespace get a silent empty slice — list is a
// filter, not a guarded read.
func (s *Service) List(ctx context.Context, namespace string) ([]*domain.SchemaAttachment, error) {
	info, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	if !s.pdp.HasNamespace(info.UserID, namespace, domain.ActionRead) {
		return nil, nil
	}

	schemas, err := s.store.List(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}

	return schemas, nil
}
