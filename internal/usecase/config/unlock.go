package config

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_unlock.go -package=config_mock . unlockEnforcer,UnlockStore,UnlockNotifier

type unlockEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type UnlockStore interface {
	UnlockConfig(ctx context.Context, namespace, path string) error
	Get(ctx context.Context, path, namespace string) (*domain.Config, error)
}

type UnlockNotifier interface {
	NotifyConfigUnlocked(ctx context.Context, cfg *domain.Config)
}

type UnlockUseCase struct {
	enforcer unlockEnforcer
	store    UnlockStore
	notifier UnlockNotifier
}

func NewUnlockUseCase(enforcer unlockEnforcer, store UnlockStore, notifier UnlockNotifier) *UnlockUseCase {
	return &UnlockUseCase{enforcer: enforcer, store: store, notifier: notifier}
}

func (uc *UnlockUseCase) Execute(ctx context.Context, namespace, path string) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, namespace, "config", "write")
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

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
