package dashboard

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=dashboard_mock -source=service.go

type (
	// enforcer checks authorization for dashboard operations.
	enforcer interface {
		Enforce(subject, domain, object, action string) (bool, error)
	}

	// nsLister returns the flat list of namespaces (names only, no config count needed here).
	nsLister interface {
		ListAll(ctx context.Context) ([]*domain.Namespace, error)
	}

	// configCounter counts configs per namespace and exposes the global revision.
	configCounter interface {
		CountByNamespace(ctx context.Context, namespace string) (int, error)
		CurrentRevision(ctx context.Context) (int64, error)
	}

	// activitySource returns the most recent changelog entries, newest first.
	activitySource interface {
		ListRecentChanges(ctx context.Context, limit int) ([]*domain.ChangelogEntry, error)
	}

	// activeClientsSource returns the snapshot of currently-connected clients.
	activeClientsSource interface {
		ListActive() []*domain.Client
	}
)

// Service provides data for the dashboard page.
type Service struct {
	enforcer   enforcer
	namespaces nsLister
	configs    configCounter
	activity   activitySource
	clients    activeClientsSource
}

// New creates a new dashboard Service.
func New(
	enforcer enforcer,
	namespaces nsLister,
	configs configCounter,
	activity activitySource,
	clients activeClientsSource,
) *Service {
	return &Service{
		enforcer:   enforcer,
		namespaces: namespaces,
		configs:    configs,
		activity:   activity,
		clients:    clients,
	}
}

// StatsResult is the aggregated dashboard summary.
type StatsResult struct {
	NamespaceCount    int
	ConfigCount       int
	ActiveClientCount int
	GlobalRevision    int64
}
