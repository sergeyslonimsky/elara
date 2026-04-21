package config

import (
	"context"
	"fmt"
)

type LockStore interface {
	LockConfig(ctx context.Context, namespace, path string) error
}

type LockUseCase struct {
	store LockStore
}

func NewLockUseCase(store LockStore) *LockUseCase {
	return &LockUseCase{store: store}
}

func (uc *LockUseCase) Execute(ctx context.Context, namespace, path string) error {
	if err := uc.store.LockConfig(ctx, namespace, path); err != nil {
		return fmt.Errorf("lock config: %w", err)
	}

	return nil
}
