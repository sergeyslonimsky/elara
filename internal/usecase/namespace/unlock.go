package namespace

import (
	"context"
	"fmt"
)

type nsUnlocker interface {
	UnlockNamespace(ctx context.Context, name string) error
}

type UnlockUseCase struct{ store nsUnlocker }

func NewUnlockUseCase(store nsUnlocker) *UnlockUseCase {
	return &UnlockUseCase{store: store}
}

func (uc *UnlockUseCase) Execute(ctx context.Context, name string) error {
	if err := uc.store.UnlockNamespace(ctx, name); err != nil {
		return fmt.Errorf("unlock namespace: %w", err)
	}

	return nil
}
