package clients

import (
	"context"
	"fmt"
	"sort"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_clients.go -package=clients_mock . ActiveSource,HistorySource

// clientsEnforcer checks authorization for client operations.
type clientsEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

// ActiveSource is the live in-memory state (typically *monitor.Registry).
type ActiveSource interface {
	ListActive() []*domain.Client
	Get(connID string) *domain.Client
	RecentEvents(connID string) []domain.ClientEvent
	Subscribe() (<-chan domain.ClientChange, func())
	SubscribeClient(connID string) (<-chan domain.ClientChange, func())
}

// HistorySource is the persistent connection log (typically *monitor.HistoryStore).
type HistorySource interface {
	List(ctx context.Context, limit int) ([]*domain.Client, error)
	ListByClient(ctx context.Context, clientName, k8sNamespace string, limit int) ([]*domain.Client, error)
}

// UseCase aggregates queries needed by the ClientsService handler.
type UseCase struct {
	enforcer clientsEnforcer
	active   ActiveSource
	history  HistorySource
}

func NewUseCase(enforcer clientsEnforcer, active ActiveSource, history HistorySource) *UseCase {
	return &UseCase{enforcer: enforcer, active: active, history: history}
}

// ListActive returns all currently-connected clients sorted by ConnectedAt
// ascending (oldest first — UI typically reverses).
func (uc *UseCase) ListActive(ctx context.Context) ([]*domain.Client, error) {
	if err := auth.CheckAccess(ctx, uc.enforcer, auth.ObjectAll, auth.ObjectClient, auth.ActionRead); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	clients := uc.active.ListActive()
	sort.Slice(clients, func(i, j int) bool {
		return clients[i].ConnectedAt.Before(clients[j].ConnectedAt)
	})

	return clients, nil
}

// Get returns one client (active or historical) plus its recent events.
//
// Lookup order:
//  1. active registry (returns recent events too)
//  2. history store, scanning newest-first up to a hard cap
//
// For historical clients, recentEvents will be nil — events are not persisted.
func (uc *UseCase) Get(
	ctx context.Context,
	id string,
) (*domain.Client, []domain.ClientEvent, error) {
	if err := auth.CheckAccess(ctx, uc.enforcer, auth.ObjectAll, auth.ObjectClient, auth.ActionRead); err != nil {
		return nil, nil, fmt.Errorf("check access: %w", err)
	}

	if c := uc.active.Get(id); c != nil {
		return c, uc.active.RecentEvents(id), nil
	}

	// Historical lookup. We don't have an indexed Get-by-ID in the history
	// repo, so we scan the most recent N. For typical UI usage (operator
	// clicked through a recent disconnection) this is fast enough.
	const historyScanLimit = 1000

	hist, err := uc.history.List(ctx, historyScanLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("list historical clients: %w", err)
	}

	for _, c := range hist {
		if c.ID == id {
			return c, nil, nil
		}
	}

	return nil, nil, nil
}

// ListHistorical returns past connections, newest first, capped at limit
// (0 → server-default cap).
func (uc *UseCase) ListHistorical(ctx context.Context, limit int) ([]*domain.Client, error) {
	if err := auth.CheckAccess(ctx, uc.enforcer, auth.ObjectAll, auth.ObjectClient, auth.ActionRead); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	const defaultLimit = 100
	if limit <= 0 {
		limit = defaultLimit
	}

	out, err := uc.history.List(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list historical connections: %w", err)
	}

	return out, nil
}

// SubscribeChanges checks authorization and returns the change channel.
func (uc *UseCase) SubscribeChanges(ctx context.Context) (<-chan domain.ClientChange, func(), error) {
	if err := auth.CheckAccess(ctx, uc.enforcer, auth.ObjectAll, auth.ObjectClient, auth.ActionRead); err != nil {
		return nil, nil, fmt.Errorf("check access: %w", err)
	}

	ch, cancel := uc.active.Subscribe()

	return ch, cancel, nil
}

// SubscribeClient checks authorization and returns the per-client change channel.
func (uc *UseCase) SubscribeClient(ctx context.Context, connID string) (<-chan domain.ClientChange, func(), error) {
	if err := auth.CheckAccess(ctx, uc.enforcer, auth.ObjectAll, auth.ObjectClient, auth.ActionRead); err != nil {
		return nil, nil, fmt.Errorf("check access: %w", err)
	}

	ch, cancel := uc.active.SubscribeClient(connID)

	return ch, cancel, nil
}

// ListSessions returns past connections of the same logical client
// (matched by client_name + k8s_namespace), excluding the optional currentID
// (typically the active session that is being viewed).
//
// Empty client_name → returns no sessions: anonymous clients can't be
// correlated across reconnects.
func (uc *UseCase) ListSessions(
	ctx context.Context,
	clientName, k8sNamespace, currentID string,
	limit int,
) ([]*domain.Client, error) {
	if err := auth.CheckAccess(ctx, uc.enforcer, auth.ObjectAll, auth.ObjectClient, auth.ActionRead); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	const defaultLimit = 50

	if clientName == "" {
		return nil, nil
	}

	if limit <= 0 {
		limit = defaultLimit
	}

	// Over-fetch by 1 to keep limit accurate after we filter out currentID.
	results, err := uc.history.ListByClient(ctx, clientName, k8sNamespace, limit+1)
	if err != nil {
		return nil, fmt.Errorf("list sessions by client: %w", err)
	}

	if currentID == "" {
		if len(results) > limit {
			results = results[:limit]
		}

		return results, nil
	}

	out := make([]*domain.Client, 0, len(results))
	for _, c := range results {
		if c.ID == currentID {
			continue
		}

		out = append(out, c)

		if len(out) >= limit {
			break
		}
	}

	return out, nil
}
