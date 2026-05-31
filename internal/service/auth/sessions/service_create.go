package sessions

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Create opens a new session for the given user and records a Created event.
// TTL is determined by ClientType: web → 8 h, cli → 30 d.
// Both the session row and the event should be written in a single transaction
// (orchestrated by the caller).
func (s *Service) Create(ctx context.Context, params CreateParams) (*domain.Session, error) {
	id, err := domain.NewSessionID()
	if err != nil {
		return nil, fmt.Errorf("sessions: create: %w", err)
	}

	eventID, err := domain.NewEventID()
	if err != nil {
		return nil, fmt.Errorf("sessions: create: %w", err)
	}

	now := s.clock.Now()

	ttl := defaultWebTTL
	if domain.ClientType(params.ClientType) == domain.ClientTypeCLI {
		ttl = defaultCLITTL
	}

	sess := &domain.Session{
		ID:         id,
		UserID:     params.UserID,
		ClientType: domain.ClientType(params.ClientType),
		IP:         params.IP,
		UserAgent:  params.UserAgent,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(ttl),
	}

	event := &domain.SessionEvent{
		ID:        eventID,
		SessionID: id,
		UserID:    params.UserID,
		Type:      domain.SessionEventCreated,
		IP:        params.IP,
		UserAgent: params.UserAgent,
		Timestamp: now,
	}

	if err := s.repo.Create(ctx, sess); err != nil {
		return nil, fmt.Errorf("sessions: create: %w", err)
	}

	if err := s.events.Append(ctx, event); err != nil {
		return nil, fmt.Errorf("sessions: create: append event: %w", err)
	}

	return sess, nil
}
