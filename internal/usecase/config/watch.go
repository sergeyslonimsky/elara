package config

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type watchSubscriber interface {
	Subscribe(ctx context.Context, pathPrefix, namespace string) (<-chan domain.WatchEvent, func())
}

type WatchUseCase struct {
	watch watchSubscriber
}

func NewWatchUseCase(watch watchSubscriber) *WatchUseCase {
	return &WatchUseCase{watch: watch}
}

func (uc *WatchUseCase) Execute(ctx context.Context, pathPrefix, namespace string) (<-chan domain.WatchEvent, func()) {
	if pathPrefix == "" {
		pathPrefix = "/"
	}

	return uc.watch.Subscribe(ctx, pathPrefix, namespace)
}
