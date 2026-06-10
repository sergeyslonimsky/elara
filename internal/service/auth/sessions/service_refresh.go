package sessions

import (
	"context"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Refresh updates LastSeenAt and, for web sessions, extends ExpiresAt under
// the sliding-TTL policy. It is a no-op when the session was seen recently
// (within refreshThrottle). CLI sessions get only LastSeenAt updated — no
// sliding extension.
func (s *Service) Refresh(ctx context.Context, id string) error {
	sess, err := s.repo.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("sessions: refresh: %w", err)
	}

	now := s.clock.Now()

	// Throttle: skip the write if the session was touched recently.
	if now.Sub(sess.LastSeenAt) < refreshThrottle {
		return nil
	}

	appendEvent := s.applySliding(sess, now)
	sess.LastSeenAt = now

	return s.persistRefresh(ctx, sess, now, appendEvent)
}

// applySliding extends sess.ExpiresAt for web sessions under the sliding-TTL
// policy and returns true when an extension occurred (caller should append an
// audit event). CLI sessions are unchanged.
func (s *Service) applySliding(sess *domain.Session, now time.Time) bool {
	if sess.ClientType != domain.ClientTypeWeb {
		return false
	}

	newExpires := now.Add(defaultWebTTL)
	hardCap := sess.CreatedAt.Add(maxWebSlidingTTL)

	if newExpires.After(hardCap) {
		newExpires = hardCap
	}

	if newExpires.Sub(sess.ExpiresAt) <= refreshMinDelta {
		return false
	}

	sess.ExpiresAt = newExpires

	return true
}

func (s *Service) persistRefresh(
	ctx context.Context,
	sess *domain.Session,
	now time.Time,
	appendEvent bool,
) error {
	if err := s.repo.Update(ctx, sess); err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	if !appendEvent {
		return nil
	}

	eventID, err := domain.NewEventID()
	if err != nil {
		return fmt.Errorf("generate event id: %w", err)
	}

	event := &domain.SessionEvent{
		ID:        eventID,
		SessionID: sess.ID,
		UserID:    sess.UserID,
		Type:      domain.SessionEventRefreshed,
		Timestamp: now,
	}

	if err = s.events.Append(ctx, event); err != nil {
		return fmt.Errorf("append refreshed event: %w", err)
	}

	return nil
}
