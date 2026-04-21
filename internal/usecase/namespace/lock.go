package namespace

import (
	"context"
	"fmt"
)

type nsLocker interface {
	LockNamespace(ctx context.Context, name string) error
}

type LockUseCase struct{ store nsLocker }

func NewLockUseCase(store nsLocker) *LockUseCase {
	return &LockUseCase{store: store}
}

func (uc *LockUseCase) Execute(ctx context.Context, name string) error {
	if err := uc.store.LockNamespace(ctx, name); err != nil {
		return fmt.Errorf("lock namespace: %w", err)
	}

	return nil
}
