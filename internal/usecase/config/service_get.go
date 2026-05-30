package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type GetInput struct {
	Namespace string
	Path      string
}

func (s *Service) Get(ctx context.Context, in GetInput) (*domain.Config, error) {
	if in.Namespace == "" {
		return nil, domain.NewValidationError("namespace", "namespace is required")
	}

	cfg, err := s.storage.Get(ctx, in.Path, in.Namespace)
	if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}

	return cfg, nil
}

type GetAtRevisionInput struct {
	Namespace string
	Path      string
	Revision  int64
}

func (s *Service) GetAtRevision(
	ctx context.Context,
	in GetAtRevisionInput,
) (*domain.HistoryEntry, error) {
	if in.Namespace == "" {
		return nil, domain.NewValidationError("namespace", "namespace is required")
	}

	entry, err := s.storage.GetAtRevision(ctx, in.Path, in.Namespace, in.Revision)
	if err != nil {
		return nil, fmt.Errorf("get config at revision: %w", err)
	}

	return entry, nil
}
