package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/content"
)

func (s *Service) Update(ctx context.Context, cfg *domain.Config) (*domain.Config, error) {
	if err := domain.ValidatePath(cfg.Path); err != nil {
		return nil, fmt.Errorf("validate path: %w", err)
	}

	// Format is immutable on update (simplifies versioning and schema tracking).
	existing, err := s.storage.Get(ctx, cfg.Path, cfg.Namespace)
	if err != nil {
		return nil, fmt.Errorf("get existing: %w", mapStorageErr(err, cfg.Path))
	}

	cfg.Format = existing.Format

	if err := content.Validate(cfg.Content, cfg.Format); err != nil {
		return nil, fmt.Errorf("validate content: %w", err)
	}

	normalized, err := content.Normalize(cfg.Content, cfg.Format)
	if err != nil {
		return nil, fmt.Errorf("normalize content: %w", err)
	}

	cfg.Content = normalized
	cfg.GenerateHash()

	if err := s.schemaValidator.Validate(ctx, cfg.Namespace, cfg.Path, cfg.Content, cfg.Format); err != nil {
		return nil, fmt.Errorf("schema validation: %w", err)
	}

	err = s.txm.WithTx(ctx, func(ctx context.Context) error {
		if err := s.storage.Update(ctx, cfg); err != nil {
			return fmt.Errorf("update config: %w", mapStorageErr(err, cfg.Path))
		}

		// namespace timestamp is cosmetic; failure must not abort the config write.
		_ = s.namespaceProvider.UpdateTimestamp(ctx, cfg.Namespace)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update config tx: %w", err)
	}

	s.watcher.NotifyUpdated(ctx, cfg)

	return cfg, nil
}

type LockInput struct {
	Namespace string
	Path      string
}

func (s *Service) Lock(ctx context.Context, in LockInput) error {
	var cfg *domain.Config

	err := s.txm.WithTx(ctx, func(ctx context.Context) error {
		if err := s.storage.LockConfig(ctx, in.Namespace, in.Path); err != nil {
			return fmt.Errorf("lock: %w", mapStorageErr(err, in.Path))
		}

		c, err := s.storage.Get(ctx, in.Path, in.Namespace)
		if err != nil {
			return fmt.Errorf("get config after lock: %w", mapStorageErr(err, in.Path))
		}
		cfg = c

		return nil
	})
	if err != nil {
		return fmt.Errorf("lock config tx: %w", err)
	}

	s.watcher.NotifyConfigLocked(ctx, cfg)

	return nil
}

type UnlockInput struct {
	Namespace string
	Path      string
}

func (s *Service) Unlock(ctx context.Context, in UnlockInput) error {
	var cfg *domain.Config

	err := s.txm.WithTx(ctx, func(ctx context.Context) error {
		if err := s.storage.UnlockConfig(ctx, in.Namespace, in.Path); err != nil {
			return fmt.Errorf("unlock: %w", mapStorageErr(err, in.Path))
		}

		c, err := s.storage.Get(ctx, in.Path, in.Namespace)
		if err != nil {
			return fmt.Errorf("get config after unlock: %w", mapStorageErr(err, in.Path))
		}
		cfg = c

		return nil
	})
	if err != nil {
		return fmt.Errorf("unlock config tx: %w", err)
	}

	s.watcher.NotifyConfigUnlocked(ctx, cfg)

	return nil
}
