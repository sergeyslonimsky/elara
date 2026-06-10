package schema

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

func (s *Service) Get(
	ctx context.Context,
	namespace, pathPattern string,
) (*domain.SchemaAttachment, error) {
	attachment, err := s.store.Get(ctx, namespace, pathPattern)
	if err != nil {
		if errors.Is(err, storage.ErrResourceNotFound) {
			return nil, fmt.Errorf("get schema: %w", domain.ErrNotFound)
		}

		return nil, fmt.Errorf("get schema: %w", err)
	}

	return attachment, nil
}

func (s *Service) GetEffective(
	ctx context.Context,
	namespace, path string,
) (*domain.SchemaAttachment, error) {
	schemas, err := s.store.List(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}

	return findBestMatch(schemas, path), nil
}
