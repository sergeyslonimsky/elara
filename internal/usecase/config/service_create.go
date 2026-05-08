package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

func (s *Service) Create(ctx context.Context, cfg *domain.Config) (*domain.Config, error) {
	if err := auth.CheckAccess(ctx, s.enforcer, cfg.Namespace, auth.ObjectConfig, auth.ActionWrite); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	if err := domain.ValidatePath(cfg.Path); err != nil {
		return nil, fmt.Errorf("validate path: %w", err)
	}

	cfg.SetDefaults()

	// Check namespace exists.
	if _, err := s.namespaceProvider.Get(ctx, cfg.Namespace); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewValidationError(
				"namespace",
				fmt.Sprintf("namespace %q does not exist", cfg.Namespace),
			)
		}

		return nil, fmt.Errorf("check namespace: %w", err)
	}

	// Auto-detect format from path extension if not specified.
	if cfg.Format == "" {
		cfg.Format = domain.DetectFormatFromPath(cfg.Path)
	}

	if err := domain.ValidateContent(cfg.Content, cfg.Format); err != nil {
		return nil, fmt.Errorf("validate content: %w", err)
	}

	normalized, err := domain.NormalizeContent(cfg.Content, cfg.Format)
	if err != nil {
		return nil, fmt.Errorf("normalize content: %w", err)
	}

	cfg.Content = normalized
	cfg.GenerateHash()
	cfg.Version = 1

	if err := s.schemaValidator.Validate(ctx, cfg.Namespace, cfg.Path, cfg.Content, cfg.Format); err != nil {
		return nil, fmt.Errorf("schema validation: %w", err)
	}

	if err := s.storage.Create(ctx, cfg); err != nil {
		return nil, fmt.Errorf("create config: %w", err)
	}

	// best-effort: namespace timestamp is cosmetic; failure must not abort the config write.
	_ = s.namespaceProvider.UpdateTimestamp(ctx, cfg.Namespace)
	s.watcher.NotifyCreated(ctx, cfg)

	return cfg, nil
}
