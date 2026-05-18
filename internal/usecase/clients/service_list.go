package clients

import (
	"context"
	"fmt"
	"sort"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

// ListActive returns currently-connected clients sorted by ConnectedAt
// ascending. Non-admin callers only see clients whose active watches touch
// at least one namespace they can read.
func (s *Service) ListActive(ctx context.Context) ([]*domain.Client, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	scope := newScopeChecker(s.enforcer, claims.Email)

	clients := s.active.ListActive()
	sort.Slice(clients, func(i, j int) bool {
		return clients[i].ConnectedAt.Before(clients[j].ConnectedAt)
	})

	return scope.filter(clients), nil
}

// ListHistorical returns past connections, newest first, capped at limit
// (0 → server-default cap). Non-admins receive an empty list because
// historical entries do not retain per-watch namespace info to scope on.
func (s *Service) ListHistorical(ctx context.Context, limit int) ([]*domain.Client, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	scope := newScopeChecker(s.enforcer, claims.Email)

	const defaultLimit = 100
	if limit <= 0 {
		limit = defaultLimit
	}

	out, err := s.history.List(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list historical connections: %w", err)
	}

	return scope.filter(out), nil
}

// ListSessions returns past connections of the same logical client
// (matched by client_name + k8s_namespace), excluding the optional currentID
// (typically the active session that is being viewed).
//
// Empty client_name → returns no sessions: anonymous clients can't be
// correlated across reconnects.
func (s *Service) ListSessions(
	ctx context.Context,
	clientName, k8sNamespace, currentID string,
	limit int,
) ([]*domain.Client, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	scope := newScopeChecker(s.enforcer, claims.Email)

	const defaultLimit = 50

	if clientName == "" {
		return nil, nil
	}

	if limit <= 0 {
		limit = defaultLimit
	}

	// Over-fetch by 1 to keep limit accurate after we filter out currentID.
	results, err := s.history.ListByClient(ctx, clientName, k8sNamespace, limit+1)
	if err != nil {
		return nil, fmt.Errorf("list sessions by client: %w", err)
	}

	if currentID != "" {
		filtered := make([]*domain.Client, 0, len(results))
		for _, c := range results {
			if c.ID == currentID {
				continue
			}

			filtered = append(filtered, c)
		}

		results = filtered
	}

	if len(results) > limit {
		results = results[:limit]
	}

	return scope.filter(results), nil
}
