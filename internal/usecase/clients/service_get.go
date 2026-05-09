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
func (s *Service) Get(
	ctx context.Context,
	id string,
) (*domain.Client, []domain.ClientEvent, error) {
	if err := auth.CheckAccess(ctx, s.enforcer, auth.ObjectAll, auth.ObjectClient, auth.ActionRead); err != nil {
		return nil, nil, fmt.Errorf("check access: %w", err)
	}

	if c := s.active.Get(id); c != nil {
		return c, s.active.RecentEvents(id), nil
	}

	hist, err := s.history.List(ctx, historyScanLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("list historical clients: %w", err)
	}

	for _, c := range hist {
		if c.ID == id {
			return c, nil, nil
		}
	}

	return nil, nil, domain.ErrNotFound
}
