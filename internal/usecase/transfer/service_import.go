package transfer

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
)

// requireTransferWrite returns ErrForbidden when the caller lacks
// (Transfer, Write) on the given namespace. Handler-level Require only covers
// the request-supplied namespace; this gate covers the per-bundle namespaces
// resolved inside Import.
func (s *Service) requireTransferWrite(ctx context.Context, namespace string) error {
	info, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return domain.ErrUnauthorized
	}

	if !s.pdp.HasNamespace(info.UserID, namespace, domain.ActionWrite) {
		return domain.ErrForbidden
	}

	return nil
}

func (s *Service) Import(
	ctx context.Context,
	data []byte,
	onConflict transferv1.ConflictResolution,
	dryRun bool,
	targetNamespace string,
) (*domain.ImportReport, error) {
	if onConflict == transferv1.ConflictResolution_CONFLICT_RESOLUTION_UNSPECIFIED {
		onConflict = transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP
	}

	if targetNamespace != "" {
		return s.importToTarget(ctx, data, onConflict, dryRun, targetNamespace)
	}

	return s.importInferredScope(ctx, data, onConflict, dryRun)
}

// importToTarget imports a single NamespaceBundle into an explicit target
// namespace. The bundle's own namespace field is overwritten.
func (s *Service) importToTarget(
	ctx context.Context,
	data []byte,
	onConflict transferv1.ConflictResolution,
	dryRun bool,
	targetNamespace string,
) (*domain.ImportReport, error) {
	bundle, err := unmarshalNamespaceBundle(data)
	if err != nil {
		return nil, domain.NewValidationError("data", fmt.Sprintf("parse bundle: %s", err))
	}

	bundle.Namespace = targetNamespace
	report := &domain.ImportReport{DryRun: dryRun}

	return report, s.importNamespaceBundle(ctx, bundle, onConflict, dryRun, report)
}

// importInferredScope tries AllBundle first (multi-namespace import) and
// falls back to a single NamespaceBundle that names its own namespace.
func (s *Service) importInferredScope(
	ctx context.Context,
	data []byte,
	onConflict transferv1.ConflictResolution,
	dryRun bool,
) (*domain.ImportReport, error) {
	allBundle, err := unmarshalAllBundle(data)
	if err != nil {
		return nil, domain.NewValidationError("data", fmt.Sprintf("parse bundle: %s", err))
	}

	if len(allBundle.Namespaces) > 0 {
		if err := s.requireTransferWrite(ctx, domain.DomainAll); err != nil {
			return nil, err
		}

		return s.importAllBundle(ctx, allBundle, onConflict, dryRun)
	}

	bundle, err := unmarshalNamespaceBundle(data)
	if err != nil {
		return nil, domain.NewValidationError("data", fmt.Sprintf("parse bundle: %s", err))
	}

	if bundle.Namespace == "" {
		return nil, domain.NewValidationError("namespace", "bundle namespace is required")
	}

	if err := s.requireTransferWrite(ctx, bundle.Namespace); err != nil {
		return nil, err
	}

	report := &domain.ImportReport{DryRun: dryRun}

	return report, s.importNamespaceBundle(ctx, bundle, onConflict, dryRun, report)
}

func (s *Service) importAllBundle(
	ctx context.Context,
	allBundle *domain.AllBundle,
	onConflict transferv1.ConflictResolution,
	dryRun bool,
) (*domain.ImportReport, error) {
	report := &domain.ImportReport{DryRun: dryRun}

	for i := range allBundle.Namespaces {
		if err := s.importNamespaceBundle(ctx, &allBundle.Namespaces[i], onConflict, dryRun, report); err != nil {
			return nil, err
		}
	}

	return report, nil
}

func (s *Service) importNamespaceBundle(
	ctx context.Context,
	bundle *domain.NamespaceBundle,
	onConflict transferv1.ConflictResolution,
	dryRun bool,
	report *domain.ImportReport,
) error {
	if !dryRun {
		if err := s.ensureNamespace(ctx, bundle.Namespace, bundle.Description); err != nil {
			return fmt.Errorf("ensure namespace: %w", err)
		}
	}

	for i := range bundle.Configs {
		if err := s.importConfig(ctx, &bundle.Configs[i], bundle.Namespace, onConflict, dryRun, report); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) importConfig(
	ctx context.Context,
	bc *domain.BundleConfig,
	namespace string,
	onConflict transferv1.ConflictResolution,
	dryRun bool,
	report *domain.ImportReport,
) error {
	existing, err := s.configs.Get(ctx, bc.Path, namespace)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("check config %s: %w", bc.Path, err)
	}

	if existing != nil {
		return s.handleConflict(ctx, bc, existing, namespace, onConflict, dryRun, report)
	}

	// New config — dry-run only counts it, real run creates it.
	if dryRun {
		report.Created++

		return nil
	}

	cfg := bundleConfigToDomain(bc, namespace)
	if err := s.configs.Create(ctx, cfg); err != nil {
		report.AddError(bc.Path, namespace, err.Error())
	} else {
		report.Created++
	}

	return nil
}

func (s *Service) handleConflict(
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
			report.Updated++

			return nil
		}

		cfg := bundleConfigToDomain(bc, namespace)
		cfg.Version = existing.Version

		if err := s.configs.Update(ctx, cfg); err != nil {
			report.AddError(bc.Path, namespace, err.Error())
		} else {
			report.Updated++
		}
	}

	return nil
}

func (s *Service) ensureNamespace(ctx context.Context, name, description string) error {
	_, err := s.namespaces.Get(ctx, name)
	if err == nil {
		return nil
	}

	if !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("get namespace: %w", err)
	}

	if err := s.namespaces.Create(ctx, &domain.Namespace{Name: name, Description: description}); err != nil {
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
