package config

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type UnlockStore interface {
	UnlockConfig(ctx context.Context, namespace, path string) error
	Get(ctx context.Context, path, namespace string) (*domain.Config, error)
}

type UnlockNotifier interface {
	NotifyConfigUnlocked(ctx context.Context, cfg *domain.Config)
}

type UnlockUseCase struct {
	store    UnlockStore
	notifier UnlockNotifier
}

func NewUnlockUseCase(store UnlockStore, notifier UnlockNotifier) *UnlockUseCase {
	return &UnlockUseCase{store: store, notifier: notifier}
}

func (uc *UnlockUseCase) Execute(ctx context.Context, namespace, path string) error {
	if err := uc.store.UnlockConfig(ctx, namespace, path); err != nil {
		return fmt.Errorf("unlock config: %w", err)
	}

	cfg, err := uc.store.Get(ctx, path, namespace)
	if err != nil {
		slog.Warn("unlock succeeded but post-unlock read failed; emitting event without config payload",
			"namespace", namespace, "path", path, "err", err)
		uc.notifier.NotifyConfigUnlocked(ctx, &domain.Config{Path: path, Namespace: namespace})

		return nil
	}

	uc.notifier.NotifyConfigUnlocked(ctx, cfg)

	return nil
}
