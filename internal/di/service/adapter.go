package service

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/sergeyslonimsky/core/lifecycle"

	bboltadapter "github.com/sergeyslonimsky/elara/internal/adapter/bbolt"
	watchadapter "github.com/sergeyslonimsky/elara/internal/adapter/watch"
	webhookadapter "github.com/sergeyslonimsky/elara/internal/adapter/webhook"
	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/monitor"
)

type Adapters struct {
	Store             *bboltadapter.Store
	ConfigRepo        *bboltadapter.ConfigRepo
	NamespaceRepo     *bboltadapter.NamespaceRepo
	ClientHistoryRepo *bboltadapter.ClientHistoryRepo
	SchemaRepo        *bboltadapter.SchemaRepo
	WebhookRepo       *bboltadapter.WebhookRepo
	Watch             *watchadapter.Publisher
	WebhookDispatcher *webhookadapter.Dispatcher

	// Connected-clients monitor: history is wired into the registry as a
	// HistorySink so disconnects are persisted automatically.
	ClientHistory  *monitor.HistoryStore
	ClientRegistry *monitor.Registry

	// shutdownOnce guarantees Shutdown is idempotent even if it's called
	// from multiple paths (app.App's LIFO teardown + a partial-failure
	// cleanup closure returned from di.NewContainer, for instance).
	shutdownOnce sync.Once
	shutdownErr  error
}

func NewAdapters(ctx context.Context, cfg config.Config) (*Adapters, error) {
	dbPath := filepath.Join(cfg.DataPath, "elara.db")

	store, err := bboltadapter.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open bbolt store: %w", err)
	}

	clientHistoryRepo := bboltadapter.NewClientHistoryRepo(store)
	clientHistory := monitor.NewHistoryStore(ctx, monitor.HistoryConfig{
		MaxRecords: cfg.Clients.HistoryMaxRecords,
		MaxAge:     cfg.Clients.HistoryMaxAge,
	}, clientHistoryRepo)

	clientRegistry := monitor.NewRegistry(monitor.Config{
		RecentEventsCapacity: cfg.Clients.RecentEventsCapacity,
	}, clientHistory)

	watchPublisher := watchadapter.NewPublisher()
	webhookRepo := bboltadapter.NewWebhookRepo(store)
	webhookDispatcher := webhookadapter.NewDispatcher(webhookRepo, watchPublisher)

	//nolint:exhaustruct // shutdownOnce/shutdownErr have valid zero values
	return &Adapters{
		Store:             store,
		ConfigRepo:        bboltadapter.NewConfigRepo(store),
		NamespaceRepo:     bboltadapter.NewNamespaceRepo(store),
		ClientHistoryRepo: clientHistoryRepo,
		SchemaRepo:        bboltadapter.NewSchemaRepo(store),
		WebhookRepo:       webhookRepo,
		Watch:             watchPublisher,
		WebhookDispatcher: webhookDispatcher,
		ClientHistory:     clientHistory,
		ClientRegistry:    clientRegistry,
	}, nil
}

// Shutdown closes every adapter in reverse dependency order. Idempotent
// and concurrent-safe: runs exactly once, subsequent calls return the
// same cached result.
func (a *Adapters) Shutdown(_ context.Context) error {
	a.shutdownOnce.Do(func() {
		if a.ClientRegistry != nil {
			a.ClientRegistry.Shutdown()
		}

		if a.ClientHistory != nil {
			a.ClientHistory.Shutdown()
		}

		if a.Watch != nil {
			a.Watch.Shutdown()
		}

		if a.Store != nil {
			if err := a.Store.Close(); err != nil {
				a.shutdownErr = fmt.Errorf("close bbolt store: %w", err)
			}
		}
	})

	return a.shutdownErr
}

var _ lifecycle.Resource = (*Adapters)(nil)
