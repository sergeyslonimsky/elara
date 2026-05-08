package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
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

	if err := auth.CheckAccess(ctx, s.enforcer, in.Namespace, auth.ObjectConfig, auth.ActionRead); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	cfg, err := s.storage.Get(ctx, in.Path, in.Namespace)
	if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}

	return cfg, nil
}
