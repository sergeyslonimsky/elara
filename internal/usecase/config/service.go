package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=config_mock -source=service.go

type (
	pdp interface {
		EffectiveDomains(
			principal string,
			object domain.Object,
			action domain.Action,
		) authz.DomainSet
		EffectiveNamespaces(actor string, action domain.Action) authz.DomainSet
	}

	// configRepo is the structured config-CRUD storage surface — everything
	// except service_kv.go's raw etcd-KV path (see kvRepo below).
	configRepo interface {
		Create(ctx context.Context, cfg *domain.Config) error
		Get(ctx context.Context, path, namespace string) (*domain.Config, error)
		Update(ctx context.Context, cfg *domain.Config) error
		Delete(ctx context.Context, path, namespace string) (int64, error)
		ListSummariesByPrefix(
			ctx context.Context,
			pathPrefix, namespace string,
		) ([]*domain.ConfigSummary, error)
		GetConfigHistory(
			ctx context.Context,
			path, namespace string,
			limit int,
		) ([]*domain.HistoryEntry, error)
		GetAtRevision(
			ctx context.Context,
			path, namespace string,
			revision int64,
		) (*domain.HistoryEntry, error)
		SearchByPath(ctx context.Context, query, namespace string) ([]*domain.ConfigSummary, error)
		LockConfig(ctx context.Context, namespace, path string) error
		UnlockConfig(ctx context.Context, namespace, path string) error
	}

	// kvRepo backs the etcd-compatible gRPC API's raw KV path (see
	// service_kv.go) — separate from configRepo because it operates on raw
	// bytes/namespace-path pairs, not hydrated domain.Config, and only
	// service_kv.go's methods ever call it. Signatures mirror
	// storage/bbolt/config/etcd.go's KVRepo verbatim.
	kvRepo interface {
		CurrentRevisionValue(ctx context.Context) (int64, error)
		RangeQuery(
			ctx context.Context,
			startNS, startPath, endNS, endPath string,
			limit, revision int64,
			keysOnly bool,
		) ([]*domain.KVPair, bool, error)
		PutKey(ctx context.Context, namespace, path string, value []byte) (*domain.KVPair, int64, error)
		DeleteRangeKeys(
			ctx context.Context,
			startNS, startPath, endNS, endPath string,
			returnPrev bool,
		) ([]*domain.KVPair, int64, error)
	}

	watcher interface {
		NotifyCreated(ctx context.Context, cfg *domain.Config)
		NotifyUpdated(ctx context.Context, cfg *domain.Config)
		NotifyDeleted(ctx context.Context, path, namespace string, revision int64)
		NotifyConfigLocked(ctx context.Context, cfg *domain.Config)
		NotifyConfigUnlocked(ctx context.Context, cfg *domain.Config)
		Subscribe(
			ctx context.Context,
			pathPrefix, namespace string,
		) (<-chan domain.WatchEvent, func())
	}

	namespaceProvider interface {
		Get(ctx context.Context, name string) (*domain.Namespace, error)
		UpdateTimestamp(ctx context.Context, name string) error
	}

	schemaValidator interface {
		Validate(
			ctx context.Context,
			namespace, configPath, content string,
			format domain.Format,
		) error
	}
)

type Service struct {
	txm               storage.Manager
	pdp               pdp
	storage           configRepo
	kv                kvRepo
	watcher           watcher
	namespaceProvider namespaceProvider
	schemaValidator   schemaValidator
}

// New's repo parameter is a single value satisfying both configRepo and
// kvRepo — in production it's the same *storage/bbolt/config.Repository
// either way (see internal/di/service/services.go) — split into two typed
// Service fields so each of configRepo/kvRepo stays small and single-purpose
// instead of one large storage-surface interface.
func New(
	txm storage.Manager,
	pdp pdp,
	repo interface {
		configRepo
		kvRepo
	},
	watcher watcher,
	namespaceProvider namespaceProvider,
	schemaValidator schemaValidator,
) *Service {
	return &Service{
		txm:               txm,
		pdp:               pdp,
		storage:           repo,
		kv:                repo,
		watcher:           watcher,
		namespaceProvider: namespaceProvider,
		schemaValidator:   schemaValidator,
	}
}

func mapStorageErr(err error, path string) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, storage.ErrResourceNotFound):
		return fmt.Errorf("map storage error: %w", domain.NewNotFoundError("config", path))
	case errors.Is(err, storage.ErrResourceAlreadyExists):
		return fmt.Errorf("map storage error: %w", domain.NewAlreadyExistsError("config", path))
	default:
		return err
	}
}
