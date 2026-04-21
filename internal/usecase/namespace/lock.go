package namespace

import (
	"context"
	"fmt"
)

type nsLocker interface {
	LockNamespace(ctx context.Context, name string) error
}

type lockNotifier interface {
	NotifyNamespaceLocked(ctx context.Context, namespace string)
}

type LockUseCase struct {
	store    nsLocker
	notifier lockNotifier
}

func NewLockUseCase(store nsLocker, notifier lockNotifier) *LockUseCase {
	return &LockUseCase{store: store, notifier: notifier}
}

func (uc *LockUseCase) Execute(ctx context.Context, name string) error {
	if err := uc.store.LockNamespace(ctx, name); err != nil {
		return fmt.Errorf("lock namespace: %w", err)
	}

	uc.notifier.NotifyNamespaceLocked(ctx, name)

	return nil
}
