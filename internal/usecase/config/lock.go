package config

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_lock.go -package=config_mock . lockEnforcer,LockStore,LockNotifier

type lockEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type LockStore interface {
	LockConfig(ctx context.Context, namespace, path string) error
	Get(ctx context.Context, path, namespace string) (*domain.Config, error)
}

type LockNotifier interface {
	NotifyConfigLocked(ctx context.Context, cfg *domain.Config)
}

type LockUseCase struct {
	enforcer lockEnforcer
	store    LockStore
	notifier LockNotifier
}

func NewLockUseCase(enforcer lockEnforcer, store LockStore, notifier LockNotifier) *LockUseCase {
	return &LockUseCase{enforcer: enforcer, store: store, notifier: notifier}
}

func (uc *LockUseCase) Execute(ctx context.Context, namespace, path string) error {
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
