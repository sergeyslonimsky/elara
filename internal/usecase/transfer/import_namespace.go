package transfer

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
)

type importConfigGetter interface {
	Get(ctx context.Context, path, namespace string) (*domain.Config, error)
}

type importConfigCreator interface {
	Create(ctx context.Context, cfg *domain.Config) error
}

type importConfigUpdater interface {
	Update(ctx context.Context, cfg *domain.Config) error
}

type importNSGetter interface {
	Get(ctx context.Context, name string) (*domain.Namespace, error)
}

type importNSCreator interface {
	Create(ctx context.Context, ns *domain.Namespace) error
}

type ImportNamespaceUseCase struct {
	configs   importConfigGetter
	creator   importConfigCreator
	updater   importConfigUpdater
	nsGetter  importNSGetter
	nsCreator importNSCreator
}

func NewImportNamespaceUseCase(
	configs importConfigGetter,
	creator importConfigCreator,
	updater importConfigUpdater,
	nsGetter importNSGetter,
	nsCreator importNSCreator,
) *ImportNamespaceUseCase {
	return &ImportNamespaceUseCase{
		configs:   configs,
		creator:   creator,
		updater:   updater,
		nsGetter:  nsGetter,
		nsCreator: nsCreator,
	}
}

func (uc *ImportNamespaceUseCase) Execute(
	ctx context.Context,
	data []byte,
	onConflict transferv1.ConflictResolution,
	dryRun bool,
) (*domain.ImportReport, error) {
	bundle, err := unmarshalNamespaceBundle(data)
	if err != nil {
		return nil, domain.NewValidationError("data", fmt.Sprintf("parse bundle: %s", err))
	}

	if bundle.Namespace == "" {
		bundle.Namespace = domain.DefaultNamespace
	}

	if onConflict == transferv1.ConflictResolution_CONFLICT_RESOLUTION_UNSPECIFIED {
		onConflict = transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP
	}

	if !dryRun {
		if err := uc.ensureNamespace(ctx, bundle.Namespace); err != nil {
			return nil, fmt.Errorf("ensure namespace: %w", err)
		}
	}

	report := &domain.ImportReport{DryRun: dryRun}

	for i := range bundle.Configs {
		if err := uc.importConfig(ctx, &bundle.Configs[i], bundle.Namespace, onConflict, dryRun, report); err != nil {
			return nil, err
		}
	}

	return report, nil
}

func (uc *ImportNamespaceUseCase) importConfig(
	ctx context.Context,
	bc *domain.BundleConfig,
	namespace string,
	onConflict transferv1.ConflictResolution,
	dryRun bool,
	report *domain.ImportReport,
) error {
	existing, err := uc.configs.Get(ctx, bc.Path, namespace)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("check config %s: %w", bc.Path, err)
	}

	if existing != nil {
		return uc.handleConflict(ctx, bc, existing, namespace, onConflict, dryRun, report)
	}

	// New config — dry-run only counts it, real run creates it.
	if dryRun {
		report.Created++

		return nil
	}

	cfg := bundleConfigToDomain(bc, namespace)
	if err := uc.creator.Create(ctx, cfg); err != nil {
		report.AddError(bc.Path, namespace, err.Error())
	} else {
		report.Created++
	}

	return nil
}

func (uc *ImportNamespaceUseCase) handleConflict(
	ctx context.Context,
	bc *domain.BundleConfig,
	existing *domain.Config,
	namespace string,
	onConflict transferv1.ConflictResolution,
	dryRun bool,
	report *domain.ImportReport,
) error {
	switch onConflict {
	case transferv1.ConflictResolution_CONFLICT_RESOLUTION_UNSPECIFIED,
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP:
		report.Skipped++

	case transferv1.ConflictResolution_CONFLICT_RESOLUTION_FAIL:
		report.AddError(bc.Path, namespace, "config already exists")

	case transferv1.ConflictResolution_CONFLICT_RESOLUTION_OVERWRITE:
		if dryRun {
			report.Created++

			return nil
		}

		cfg := bundleConfigToDomain(bc, namespace)
		cfg.Version = existing.Version

		if err := uc.updater.Update(ctx, cfg); err != nil {
			report.AddError(bc.Path, namespace, err.Error())
		} else {
			report.Created++
		}
	}

	return nil
}

func (uc *ImportNamespaceUseCase) ensureNamespace(ctx context.Context, name string) error {
	_, err := uc.nsGetter.Get(ctx, name)
	if err == nil {
		return nil
	}

	if !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("get namespace: %w", err)
	}

	if name == domain.DefaultNamespace {
		return nil
	}

	if err := uc.nsCreator.Create(ctx, &domain.Namespace{Name: name}); err != nil {
		return fmt.Errorf("create namespace: %w", err)
	}

	return nil
}

func bundleConfigToDomain(bc *domain.BundleConfig, namespace string) *domain.Config {
	cfg := &domain.Config{
		Path:      bc.Path,
		Content:   bc.Content,
		Format:    bc.Format,
		Namespace: namespace,
		Metadata:  bc.Metadata,
	}

	cfg.GenerateHash()

	return cfg
}
