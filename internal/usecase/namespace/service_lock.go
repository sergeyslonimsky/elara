package namespace

import (
	"context"
	"fmt"
)

// Lock and Unlock toggle the lock flag on a namespace. Authorization
// (namespace/write on `name`) is enforced at the handler boundary.

func (s *Service) Lock(ctx context.Context, name string) error {
	err := s.txm.WithTx(ctx, func(ctx context.Context) error {
		return s.store.LockNamespace(ctx, name)
	})
	if err != nil {
		return fmt.Errorf("lock namespace: %w", err)
	}

	s.notifier.NotifyNamespaceLocked(ctx, name)

	return nil
}

func (s *Service) Unlock(ctx context.Context, name string) error {
	err := s.txm.WithTx(ctx, func(ctx context.Context) error {
		return s.store.UnlockNamespace(ctx, name)
	})
	if err != nil {
		return fmt.Errorf("unlock namespace: %w", err)
	}

	s.notifier.NotifyNamespaceUnlocked(ctx, name)

	return nil
}
