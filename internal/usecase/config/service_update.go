package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func (s *Service) Update(ctx context.Context, cfg *domain.Config) (*domain.Config, error) {
	if err := auth.CheckAccess(ctx, s.enforcer, cfg.Namespace, domain.ObjectConfig, domain.ActionWrite); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	if err := domain.ValidatePath(cfg.Path); err != nil {
		return nil, fmt.Errorf("validate path: %w", err)
	}

	// Format is immutable on update (simplifies versioning and schema tracking).
	existing, err := s.storage.Get(ctx, cfg.Path, cfg.Namespace)
	if err != nil {
		return nil, fmt.Errorf("get existing: %w", err)
	}

	cfg.Format = existing.Format

	if err := domain.ValidateContent(cfg.Content, cfg.Format); err != nil {
		return nil, fmt.Errorf("validate content: %w", err)
	}

	normalized, err := domain.NormalizeContent(cfg.Content, cfg.Format)
	if err != nil {
		return nil, fmt.Errorf("normalize content: %w", err)
	}

	cfg.Content = normalized
	cfg.GenerateHash()

	if err := s.schemaValidator.Validate(ctx, cfg.Namespace, cfg.Path, cfg.Content, cfg.Format); err != nil {
		return nil, fmt.Errorf("schema validation: %w", err)
	}

	if err := s.storage.Update(ctx, cfg); err != nil {
		return nil, fmt.Errorf("update config: %w", err)
	}

	// best-effort: namespace timestamp is cosmetic; failure must not abort the config write.
	_ = s.namespaceProvider.UpdateTimestamp(ctx, cfg.Namespace)
	s.watcher.NotifyUpdated(ctx, cfg)

	return cfg, nil
}
