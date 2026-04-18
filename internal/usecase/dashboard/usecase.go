package dashboard

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// nsLister returns the flat list of namespaces (names only, no config count needed here).
type nsLister interface {
	List(ctx context.Context) ([]*domain.Namespace, error)
}

// configCounter counts configs per namespace and exposes the global revision.
type configCounter interface {
	CountByNamespace(ctx context.Context, namespace string) (int, error)
	CurrentRevision(ctx context.Context) (int64, error)
}

// activitySource returns the most recent changelog entries, newest first.
type activitySource interface {
	ListRecentChanges(ctx context.Context, limit int) ([]*domain.ChangelogEntry, error)
}

// activeClientsSource returns the snapshot of currently-connected clients.
type activeClientsSource interface {
	ListActive() []*domain.Client
}

// StatsResult is the aggregated dashboard summary.
type StatsResult struct {
	NamespaceCount    int
	ConfigCount       int
	ActiveClientCount int
	GlobalRevision    int64
}

// UseCase provides data for the dashboard page.
type UseCase struct {
	namespaces nsLister
	configs    configCounter
	activity   activitySource
	clients    activeClientsSource
}

func NewUseCase(
	namespaces nsLister,
	configs configCounter,
	activity activitySource,
	clients activeClientsSource,
) *UseCase {
	return &UseCase{
		namespaces: namespaces,
		configs:    configs,
		activity:   activity,
		clients:    clients,
	}
}

// GetStats collects KPI numbers for the dashboard header.
func (uc *UseCase) GetStats(ctx context.Context) (*StatsResult, error) {
	namespaces, err := uc.namespaces.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	var totalConfigs int
	for _, ns := range namespaces {
		count, err := uc.configs.CountByNamespace(ctx, ns.Name)
		if err != nil {
			return nil, fmt.Errorf("count configs for namespace %q: %w", ns.Name, err)
		}

		totalConfigs += count
	}

	revision, err := uc.configs.CurrentRevision(ctx)
	if err != nil {
		return nil, fmt.Errorf("get current revision: %w", err)
	}

	return &StatsResult{
		NamespaceCount:    len(namespaces),
		ConfigCount:       totalConfigs,
		ActiveClientCount: len(uc.clients.ListActive()),
		GlobalRevision:    revision,
	}, nil
}

// ListActivity returns the most recent changelog entries.
func (uc *UseCase) ListActivity(ctx context.Context, limit int) ([]*domain.ChangelogEntry, error) {
	entries, err := uc.activity.ListRecentChanges(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent changes: %w", err)
	}

	return entries, nil
}
