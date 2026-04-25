package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type configCreator interface {
	Create(ctx context.Context, cfg *domain.Config) error
}

type createWatchNotifier interface {
	NotifyCreated(ctx context.Context, cfg *domain.Config)
}

type createNSTimestampUpdater interface {
	UpdateTimestamp(ctx context.Context, name string) error
}

type createNSChecker interface {
	Get(ctx context.Context, name string) (*domain.Namespace, error)
}

type createSchemaValidator interface {
	Execute(ctx context.Context, namespace, configPath, content string, format domain.Format) error
}

type CreateUseCase struct {
	configs         configCreator
	watch           createWatchNotifier
	namespaces      createNSTimestampUpdater
	nsChecker       createNSChecker
	schemaValidator createSchemaValidator
}

func NewCreateUseCase(
	configs configCreator,
	watch createWatchNotifier,
	namespaces createNSTimestampUpdater,
	nsChecker createNSChecker,
	schemaValidator createSchemaValidator,
) *CreateUseCase {
	return &CreateUseCase{
		configs:         configs,
		watch:           watch,
		namespaces:      namespaces,
		nsChecker:       nsChecker,
		schemaValidator: schemaValidator,
	}
}

func (uc *CreateUseCase) Execute(ctx context.Context, cfg *domain.Config) (*domain.Config, error) {
	if err := domain.ValidatePath(cfg.Path); err != nil {
		return nil, fmt.Errorf("validate path: %w", err)
	}

	cfg.SetDefaults()

	// Check namespace exists.
	if _, err := uc.nsChecker.Get(ctx, cfg.Namespace); err != nil {
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

	if err := uc.schemaValidator.Execute(ctx, cfg.Namespace, cfg.Path, cfg.Content, cfg.Format); err != nil {
		return nil, fmt.Errorf("schema validation: %w", err)
	}

	if err := uc.configs.Create(ctx, cfg); err != nil {
		return nil, fmt.Errorf("create config: %w", err)
	}

	// best-effort: namespace timestamp is cosmetic; failure must not abort the config write.
	_ = uc.namespaces.UpdateTimestamp(ctx, cfg.Namespace)
	uc.watch.NotifyCreated(ctx, cfg)

	return cfg, nil
}
