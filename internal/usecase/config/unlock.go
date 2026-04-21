package config

import (
	"context"
	"fmt"
)

type UnlockStore interface {
	UnlockConfig(ctx context.Context, namespace, path string) error
}

type UnlockUseCase struct {
	store UnlockStore
}

func NewUnlockUseCase(store UnlockStore) *UnlockUseCase {
	return &UnlockUseCase{store: store}
}

func (uc *UnlockUseCase) Execute(ctx context.Context, namespace, path string) error {
	if err := uc.store.UnlockConfig(ctx, namespace, path); err != nil {
		return fmt.Errorf("unlock config: %w", err)
	}

	return nil
}
