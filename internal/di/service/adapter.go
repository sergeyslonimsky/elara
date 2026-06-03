package service

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/sergeyslonimsky/core/lifecycle"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	monitor2 "github.com/sergeyslonimsky/elara/internal/service/monitor"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	clienthistoryrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/client_history"
	namespacerepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/namespace"
	webhookrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/webhook"
	watchadapter "github.com/sergeyslonimsky/elara/internal/transport/watch"
	webhookadapter "github.com/sergeyslonimsky/elara/internal/transport/webhook"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

type Adapters struct {
	Store             *bbolt.Store
	ConfigRepo        *bbolt.ConfigRepo
	NamespaceRepo     *namespacerepo.Repository
	ClientHistoryRepo *clienthistoryrepo.Repository
	SchemaRepo        *bbolt.SchemaRepo
	WebhookRepo       *webhookrepo.Repository
	AuthUsers         *bbolt.UserRepo
	AuthGroups        *bbolt.GroupRepo
	AuthTokens        *bbolt.TokenRepo
	AuthPolicy        *bbolt.PolicyRepo
	SessionRepo       *bbolt.SessionRepo
	SessionEventRepo  *bbolt.SessionEventRepo
	Watch             *watchadapter.Publisher
	WebhookDispatcher *webhookadapter.Dispatcher
	StorageManager    storage.Manager

	// Connected-clients monitor: history is wired into the registry as a
	// HistorySink so disconnects are persisted automatically.
	ClientHistory  *monitor2.HistoryStore
	ClientRegistry *monitor2.Registry

	// shutdownOnce guarantees Shutdown is idempotent even if it's called
	// from multiple paths (app.App's LIFO teardown + a partial-failure
	// cleanup closure returned from di.NewContainer, for instance).
	shutdownOnce sync.Once
	shutdownErr  error
}

func NewAdapters(ctx context.Context, cfg config.Config) (*Adapters, error) {
	dbPath := filepath.Join(cfg.DataPath, "elara.db")

	store, err := bbolt.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open bbolt store: %w", err)
	}

	storageManager := bbolt.NewManager(store.DB())
	pkgManager := pkgbbolt.NewManager(store.DB())

	clientHistoryRepo := clienthistoryrepo.NewRepository(pkgManager)
	clientHistory := monitor2.NewHistoryStore(ctx, monitor2.HistoryConfig{
		MaxRecords: cfg.Client.History.MaxRecords,
		MaxAge:     cfg.Client.History.MaxAge,
	}, clientHistoryRepo)

	clientRegistry := monitor2.NewRegistry(monitor2.Config{
		RecentEventsCapacity: cfg.Client.RecentEvents.Capacity,
	}, clientHistory)

	watchPublisher := watchadapter.NewPublisher()
	webhookRepo := webhookrepo.NewRepository(pkgManager)
	webhookDispatcher := webhookadapter.NewDispatcher(webhookRepo, watchPublisher)

	//nolint:exhaustruct // shutdownOnce/shutdownErr have valid zero values
	return &Adapters{
		Store:             store,
		ConfigRepo:        bbolt.NewConfigRepo(storageManager),
		NamespaceRepo:     namespacerepo.NewRepository(pkgManager),
		ClientHistoryRepo: clientHistoryRepo,
		SchemaRepo:        bbolt.NewSchemaRepo(storageManager),
		WebhookRepo:       webhookRepo,
		AuthUsers:         bbolt.NewUserRepo(storageManager),
		AuthGroups:        bbolt.NewGroupRepo(storageManager),
		AuthTokens:        bbolt.NewTokenRepo(storageManager),
		AuthPolicy:        bbolt.NewPolicyRepo(storageManager),
		SessionRepo:       bbolt.NewSessionRepo(storageManager),
		SessionEventRepo:  bbolt.NewSessionEventRepo(storageManager),
		Watch:             watchPublisher,
		WebhookDispatcher: webhookDispatcher,
		StorageManager:    storageManager,
		ClientHistory:     clientHistory,
		ClientRegistry:    clientRegistry,
	}, nil
}

// Shutdown closes every adapter in reverse dependency order. Idempotent
// and concurrent-safe: runs exactly once, subsequent calls return the
// same cached result.
func (a *Adapters) Shutdown(ctx context.Context) error { //nolint:cyclop //refactor
	a.shutdownOnce.Do(func() {
		if a.WebhookDispatcher != nil {
			if err := a.WebhookDispatcher.Shutdown(ctx); err != nil {
				a.shutdownErr = fmt.Errorf("shutdown webhook dispatcher: %w", err)
			}
		}

		if a.ClientRegistry != nil {
			if err := a.ClientRegistry.Shutdown(ctx); err != nil && a.shutdownErr == nil {
				a.shutdownErr = fmt.Errorf("shutdown client registry: %w", err)
			}
		}

		if a.ClientHistory != nil {
			if err := a.ClientHistory.Shutdown(ctx); err != nil && a.shutdownErr == nil {
				a.shutdownErr = fmt.Errorf("shutdown client history: %w", err)
			}
		}

		if a.Watch != nil {
			if err := a.Watch.Shutdown(ctx); err != nil && a.shutdownErr == nil {
				a.shutdownErr = fmt.Errorf("shutdown watch: %w", err)
			}
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
