package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type configUpdater interface {
	Update(ctx context.Context, cfg *domain.Config) error
}

type updateConfigGetter interface {
	Get(ctx context.Context, path, namespace string) (*domain.Config, error)
}

type updateWatchNotifier interface {
	NotifyUpdated(ctx context.Context, cfg *domain.Config)
}

type updateNSTimestampUpdater interface {
	UpdateTimestamp(ctx context.Context, name string) error
}

type updateSchemaValidator interface {
	Execute(ctx context.Context, namespace, configPath, content string, format domain.Format) error
}

type UpdateUseCase struct {
	configs         configUpdater
	getter          updateConfigGetter
	watch           updateWatchNotifier
	namespaces      updateNSTimestampUpdater
	schemaValidator updateSchemaValidator
}

func NewUpdateUseCase(
	configs configUpdater,
	getter updateConfigGetter,
	watch updateWatchNotifier,
	namespaces updateNSTimestampUpdater,
	schemaValidator updateSchemaValidator,
) *UpdateUseCase {
	return &UpdateUseCase{
		configs:         configs,
		getter:          getter,
		watch:           watch,
		namespaces:      namespaces,
		schemaValidator: schemaValidator,
	}
}

func (uc *UpdateUseCase) Execute(ctx context.Context, cfg *domain.Config) (*domain.Config, error) {
	if err := domain.ValidatePath(cfg.Path); err != nil {
		return nil, fmt.Errorf("validate path: %w", err)
	}

	cfg.SetDefaults()

	// If format not specified, keep the existing format.
	if cfg.Format == "" {
		existing, err := uc.getter.Get(ctx, cfg.Path, cfg.Namespace)
		if err != nil {
			return nil, fmt.Errorf("get existing config: %w", err)
		}

		cfg.Format = existing.Format
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

	if err := uc.schemaValidator.Execute(ctx, cfg.Namespace, cfg.Path, cfg.Content, cfg.Format); err != nil {
		return nil, fmt.Errorf("schema validation: %w", err)
	}

	if err := uc.configs.Update(ctx, cfg); err != nil {
		return nil, fmt.Errorf("update config: %w", err)
	}

	// best-effort: namespace timestamp is cosmetic; failure must not abort the config write.
	_ = uc.namespaces.UpdateTimestamp(ctx, cfg.Namespace)
	uc.watch.NotifyUpdated(ctx, cfg)

	return cfg, nil
}
