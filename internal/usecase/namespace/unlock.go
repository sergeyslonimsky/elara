package namespace

import (
	"context"
	"fmt"
)

type nsUnlocker interface {
	UnlockNamespace(ctx context.Context, name string) error
}

type unlockNotifier interface {
	NotifyNamespaceUnlocked(ctx context.Context, namespace string)
}

type UnlockUseCase struct {
	store    nsUnlocker
	notifier unlockNotifier
}

func NewUnlockUseCase(store nsUnlocker, notifier unlockNotifier) *UnlockUseCase {
	return &UnlockUseCase{store: store, notifier: notifier}
}

func (uc *UnlockUseCase) Execute(ctx context.Context, name string) error {
	if err := uc.store.UnlockNamespace(ctx, name); err != nil {
		return fmt.Errorf("unlock namespace: %w", err)
	}

	uc.notifier.NotifyNamespaceUnlocked(ctx, name)

	return nil
}
