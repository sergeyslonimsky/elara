package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type configDeleter interface {
	Delete(ctx context.Context, path, namespace string) (int64, error)
}

type deleteWatchNotifier interface {
	NotifyDeleted(ctx context.Context, path, namespace string, revision int64)
}

type DeleteUseCase struct {
	configs configDeleter
	watch   deleteWatchNotifier
}

func NewDeleteUseCase(configs configDeleter, watch deleteWatchNotifier) *DeleteUseCase {
	return &DeleteUseCase{configs: configs, watch: watch}
}

func (uc *DeleteUseCase) Execute(ctx context.Context, path, namespace string) error {
	if namespace == "" {
		return domain.NewValidationError("namespace", "namespace is required")
	}

	rev, err := uc.configs.Delete(ctx, path, namespace)
	if err != nil {
		return fmt.Errorf("delete config: %w", err)
	}

	uc.watch.NotifyDeleted(ctx, path, namespace, rev)

	return nil
}
