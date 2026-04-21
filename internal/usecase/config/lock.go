package config

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type LockStore interface {
	LockConfig(ctx context.Context, namespace, path string) error
	Get(ctx context.Context, path, namespace string) (*domain.Config, error)
}

type LockNotifier interface {
	NotifyConfigLocked(ctx context.Context, cfg *domain.Config)
}

type LockUseCase struct {
	store    LockStore
	notifier LockNotifier
}

func NewLockUseCase(store LockStore, notifier LockNotifier) *LockUseCase {
	return &LockUseCase{store: store, notifier: notifier}
}

func (uc *LockUseCase) Execute(ctx context.Context, namespace, path string) error {
	if err := uc.store.LockConfig(ctx, namespace, path); err != nil {
		return fmt.Errorf("lock config: %w", err)
	}

	cfg, err := uc.store.Get(ctx, path, namespace)
	if err != nil {
		// Lock already committed; failing the caller would be misleading.
		// Emit the event without a full config payload so subscribers still learn about the state change.
		slog.Warn("lock succeeded but post-lock read failed; emitting event without config payload",
			"namespace", namespace, "path", path, "err", err)
		uc.notifier.NotifyConfigLocked(ctx, &domain.Config{Path: path, Namespace: namespace, Locked: true})

		return nil
	}

	uc.notifier.NotifyConfigLocked(ctx, cfg)

	return nil
}
