package sessions

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// ListMine returns active (non-revoked, non-expired) sessions for the caller.
func (s *Service) ListMine(ctx context.Context, userID string) ([]*domain.Session, error) {
	sessions, err := s.repo.ListActiveByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("sessions: list mine: %w", err)
	}

	return sessions, nil
}

// ListByUser returns all sessions (including revoked) for the target user.
// Intended for admin views.
func (s *Service) ListByUser(ctx context.Context, targetID string) ([]*domain.Session, error) {
	sessions, err := s.repo.ListByUser(ctx, targetID)
	if err != nil {
		return nil, fmt.Errorf("sessions: list by user: %w", err)
	}

	return sessions, nil
}

// ListEvents returns audit events matching the filter. Type, From, To filtering
// is not yet supported at the repo layer — only UserID-based pagination is
// implemented. Additional filter fields will be applied once the repo grows
// the corresponding index support. TODO: push Type/From/To filters down to repo.
func (s *Service) ListEvents(ctx context.Context, filter EventFilter) ([]*domain.SessionEvent, error) {
	events, err := s.events.ListByUser(ctx, filter.UserID, filter.Limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("sessions: list events: %w", err)
	}

	return events, nil
}
