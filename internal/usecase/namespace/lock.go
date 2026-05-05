package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
)

// Lock

type nsLocker interface {
	LockNamespace(ctx context.Context, name string) error
}

type lockNotifier interface {
	NotifyNamespaceLocked(ctx context.Context, namespace string)
}

type LockUseCase struct {
	enforcer auth.AccessEnforcer
	store    nsLocker
	notifier lockNotifier
}

func NewLockUseCase(enforcer auth.AccessEnforcer, store nsLocker, notifier lockNotifier) *LockUseCase {
	return &LockUseCase{enforcer: enforcer, store: store, notifier: notifier}
}

func (uc *LockUseCase) Execute(ctx context.Context, name string) error {
	// domain = namespace name itself.
	if err := auth.CheckAccess(ctx, uc.enforcer, name, auth.ObjectNamespace, auth.ActionWrite); err != nil {
		return fmt.Errorf("check access: %w", err)
	}

	if err := uc.store.LockNamespace(ctx, name); err != nil {
		return fmt.Errorf("lock namespace: %w", err)
	}

	uc.notifier.NotifyNamespaceLocked(ctx, name)

	return nil
}

// Unlock

type nsUnlocker interface {
	UnlockNamespace(ctx context.Context, name string) error
}

type unlockNotifier interface {
	NotifyNamespaceUnlocked(ctx context.Context, namespace string)
}

type UnlockUseCase struct {
	enforcer auth.AccessEnforcer
	store    nsUnlocker
	notifier unlockNotifier
}

func NewUnlockUseCase(enforcer auth.AccessEnforcer, store nsUnlocker, notifier unlockNotifier) *UnlockUseCase {
	return &UnlockUseCase{enforcer: enforcer, store: store, notifier: notifier}
}

func (uc *UnlockUseCase) Execute(ctx context.Context, name string) error {
	// domain = namespace name itself.
	if err := auth.CheckAccess(ctx, uc.enforcer, name, auth.ObjectNamespace, auth.ActionWrite); err != nil {
		return fmt.Errorf("check access: %w", err)
	}

	if err := uc.store.UnlockNamespace(ctx, name); err != nil {
		return fmt.Errorf("unlock namespace: %w", err)
	}

	uc.notifier.NotifyNamespaceUnlocked(ctx, name)

	return nil
}
