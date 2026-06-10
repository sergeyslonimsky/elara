package sessions

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Revoke terminates a single session and appends an audit event with the
// provided eventType (e.g. RevokedByUser, RevokedByAdmin).
func (s *Service) Revoke(
	ctx context.Context,
	id, revokedBy, reason string,
	eventType domain.SessionEventType,
) error {
	sess, err := s.repo.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("sessions: revoke: %w", err)
	}

	now := s.clock.Now()
	sess.RevokedAt = &now
	sess.RevokedBy = revokedBy

	eventID, err := domain.NewEventID()
	if err != nil {
		return fmt.Errorf("sessions: revoke: %w", err)
	}

	event := &domain.SessionEvent{
		ID:        eventID,
		SessionID: sess.ID,
		UserID:    sess.UserID,
		Type:      eventType,
		Reason:    reason,
		Timestamp: now,
	}

	if err := s.repo.Update(ctx, sess); err != nil {
		return fmt.Errorf("update revoked session: %w", err)
	}

	if err := s.events.Append(ctx, event); err != nil {
		return fmt.Errorf("append revoke event: %w", err)
	}

	return nil
}

// RevokeAllForUser marks every active session for userID as revoked and
// appends a RevokedCascade event for each one.
//
// Strategy: list active sessions first (within the same context/tx), then call
// RevokeAllForUser on the repo (which marks them revoked in one pass), then
// append one event per session using the IDs collected before the bulk revoke.
// Atomicity is the responsibility of the caller (M6).
func (s *Service) RevokeAllForUser(ctx context.Context, userID, revokedBy, reason string) error {
	now := s.clock.Now()

	// Collect IDs before bulk-revoking so we can emit one event per session.
	active, err := s.repo.ListActiveByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("list active sessions: %w", err)
	}

	if _, err = s.repo.RevokeAllForUser(ctx, userID, revokedBy); err != nil {
		return fmt.Errorf("bulk revoke: %w", err)
	}

	for _, sess := range active {
		eventID, err := domain.NewEventID()
		if err != nil {
			return fmt.Errorf("generate event id: %w", err)
		}

		event := &domain.SessionEvent{
			ID:        eventID,
			SessionID: sess.ID,
			UserID:    sess.UserID,
			Type:      domain.SessionEventRevokedCascade,
			Reason:    reason,
			Timestamp: now,
		}

		if err = s.events.Append(ctx, event); err != nil {
			return fmt.Errorf("append cascade revoke event for session %s: %w", sess.ID, err)
		}
	}

	return nil
}
