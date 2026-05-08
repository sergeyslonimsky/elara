package config

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/config_mock.go -package=config_mock -source=service.go

type (
	enforcer interface {
		Enforce(subject, domain, object, action string) (bool, error)
	}

	storage interface {
		Create(ctx context.Context, cfg *domain.Config) error
		Get(ctx context.Context, path, namespace string) (*domain.Config, error)
		Update(ctx context.Context, cfg *domain.Config) error
		Delete(ctx context.Context, path, namespace string) (int64, error)
		ListSummariesByPrefix(ctx context.Context, pathPrefix, namespace string) ([]*domain.ConfigSummary, error)
		GetConfigHistory(ctx context.Context, path, namespace string, limit int) ([]*domain.HistoryEntry, error)
		GetAtRevision(ctx context.Context, path, namespace string, revision int64) (*domain.HistoryEntry, error)
		SearchByPath(ctx context.Context, query, namespace string) ([]*domain.ConfigSummary, error)
		LockConfig(ctx context.Context, namespace, path string) error
		UnlockConfig(ctx context.Context, namespace, path string) error
	}

	watcher interface {
		NotifyCreated(ctx context.Context, cfg *domain.Config)
		NotifyUpdated(ctx context.Context, cfg *domain.Config)
		NotifyDeleted(ctx context.Context, path, namespace string, revision int64)
		NotifyConfigLocked(ctx context.Context, cfg *domain.Config)
		NotifyConfigUnlocked(ctx context.Context, cfg *domain.Config)
		Subscribe(ctx context.Context, pathPrefix, namespace string) (<-chan domain.WatchEvent, func())
	}

	namespaceProvider interface {
		Get(ctx context.Context, name string) (*domain.Namespace, error)
		UpdateTimestamp(ctx context.Context, name string) error
	}

	schemaValidator interface {
		Validate(ctx context.Context, namespace, configPath, content string, format domain.Format) error
	}
)

type Service struct {
	enforcer          enforcer
	storage           storage
	watcher           watcher
	namespaceProvider namespaceProvider
	schemaValidator   schemaValidator
}

func New(
	enforcer enforcer,
	storage storage,
	watcher watcher,
	namespaceProvider namespaceProvider,
	schemaValidator schemaValidator,
) *Service {
	return &Service{
		enforcer:          enforcer,
		storage:           storage,
		watcher:           watcher,
		namespaceProvider: namespaceProvider,
		schemaValidator:   schemaValidator,
	}
}
