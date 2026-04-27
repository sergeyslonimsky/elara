package service

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/di/config"
)

type Manager struct {
	Adapters     *Adapters
	UseCases     *UseCases
	V2Handlers   *V2Handlers
	EtcdHandlers *EtcdHandlers
}

// NewServiceManager implements core/di.ServicesInit. On partial-failure
// (adapters ok, later step errors out) it returns a cleanup closure that
// closes the already-opened resources — otherwise they'd leak. On success,
// core/di.NewContainer discards the cleanup; runtime teardown is driven
// by app.App via Adapters' lifecycle.Resource registration in main.
func NewServiceManager(
	ctx context.Context,
	cfg config.Config,
) (*Manager, func(context.Context) error, error) {
	adapters, err := NewAdapters(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("init adapters: %w", err)
	}

	// Cleanup closes every resource opened so far. Re-assign if subsequent
	// init steps allocate more resources — callers that grow this function
	// should keep the chain up to date.
	cleanup := func(ctx context.Context) error {
		return adapters.Shutdown(ctx)
	}

	useCases := NewUseCases(adapters)

	go adapters.WebhookDispatcher.Start(ctx)

	return &Manager{
		Adapters:     adapters,
		UseCases:     useCases,
		V2Handlers:   NewV2Handlers(useCases),
		EtcdHandlers: NewEtcdHandlers(adapters),
	}, cleanup, nil
}
