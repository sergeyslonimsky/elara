package config

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type copyConfigGetter interface {
	Get(ctx context.Context, path, namespace string) (*domain.Config, error)
}

type copyConfigCreator interface {
	Create(ctx context.Context, cfg *domain.Config) error
}

type copyWatchNotifier interface {
	NotifyCreated(ctx context.Context, cfg *domain.Config)
}

type copyNSChecker interface {
	Get(ctx context.Context, name string) (*domain.Namespace, error)
}

type copyNSTimestampUpdater interface {
	UpdateTimestamp(ctx context.Context, name string) error
}

type CopyUseCase struct {
	getter     copyConfigGetter
	creator    copyConfigCreator
	watch      copyWatchNotifier
	nsChecker  copyNSChecker
	namespaces copyNSTimestampUpdater
}

func NewCopyUseCase(
	getter copyConfigGetter,
	creator copyConfigCreator,
	watch copyWatchNotifier,
	nsChecker copyNSChecker,
	namespaces copyNSTimestampUpdater,
) *CopyUseCase {
	return &CopyUseCase{
		getter:     getter,
		creator:    creator,
		watch:      watch,
		nsChecker:  nsChecker,
		namespaces: namespaces,
	}
}

func (uc *CopyUseCase) Execute(
	ctx context.Context,
	srcPath, srcNamespace, dstPath, dstNamespace string,
) (*domain.Config, error) {
	if err := domain.ValidatePath(dstPath); err != nil {
		return nil, fmt.Errorf("validate destination path: %w", err)
	}

	if srcNamespace == "" {
		srcNamespace = domain.DefaultNamespace
	}

	if dstNamespace == "" {
		dstNamespace = domain.DefaultNamespace
	}

	// Get source config.
	source, err := uc.getter.Get(ctx, srcPath, srcNamespace)
	if err != nil {
		return nil, fmt.Errorf("get source config: %w", err)
	}

	// Check destination namespace exists.
	if _, err := uc.nsChecker.Get(ctx, dstNamespace); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewValidationError(
				"destination_namespace",
				fmt.Sprintf("namespace %q does not exist", dstNamespace),
			)
		}

		return nil, fmt.Errorf("check destination namespace: %w", err)
	}

	// Create copy at destination.
	dest := &domain.Config{
		Path:      dstPath,
		Content:   source.Content,
		Format:    source.Format,
		Namespace: dstNamespace,
		Metadata:  copyMetadata(source.Metadata),
	}

	dest.GenerateHash()
	dest.Version = 1

	if err := uc.creator.Create(ctx, dest); err != nil {
		return nil, fmt.Errorf("create copy: %w", err)
	}

	// best-effort: namespace timestamp is cosmetic; failure must not abort the config write.
	_ = uc.namespaces.UpdateTimestamp(ctx, dstNamespace)
	uc.watch.NotifyCreated(ctx, dest)

	return dest, nil
}

func copyMetadata(src map[string]string) map[string]string {
	if src == nil {
		return make(map[string]string)
	}

	dst := make(map[string]string, len(src))
	maps.Copy(dst, src)

	return dst
}
