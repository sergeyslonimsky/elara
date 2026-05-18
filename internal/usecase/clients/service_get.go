package clients

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

// Get returns one client (active or historical) plus its recent events.
//
// Lookup order:
//  1. active registry (returns recent events too)
//  2. history store, scanning newest-first up to a hard cap
//
// For historical clients, recentEvents will be nil — events are not persisted.
//
// Non-admin callers can only see clients whose active watches touch a
// namespace they can read; otherwise the response is ErrNotFound (chosen
// over ErrForbidden so the existence of the client is not revealed).
func (s *Service) Get(
	ctx context.Context,
	id string,
) (*domain.Client, []domain.ClientEvent, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, nil, domain.ErrUnauthorized
	}

	scope := newScopeChecker(s.enforcer, claims.Email)

	if c := s.active.Get(id); c != nil {
		if !scope.visible(c) {
			return nil, nil, domain.ErrNotFound
		}

		return c, s.active.RecentEvents(id), nil
	}

	hist, err := s.history.List(ctx, historyScanLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("list historical clients: %w", err)
	}

	for _, c := range hist {
		if c.ID != id {
			continue
		}

		if !scope.visible(c) {
			return nil, nil, domain.ErrNotFound
		}

		return c, nil, nil
	}

	return nil, nil, domain.ErrNotFound
}
