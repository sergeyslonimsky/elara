package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage/internal"
	"github.com/sergeyslonimsky/elara/pkg/bbolt"
)

// EventRepository is the append-only audit log for session lifecycle events.
// It writes into three buckets (primary + two secondary indexes); atomicity
// across them is the caller's responsibility (wrap Append in Manager.WithTx
// when the partial state matters).
type EventRepository struct {
	dbm bbolt.Manager
}

func NewEventRepository(dbm bbolt.Manager) *EventRepository {
	return &EventRepository{dbm: dbm}
}

// Append validates and persists a new SessionEvent. Returns a validation error
// when the event is incomplete.
func (r *EventRepository) Append(ctx context.Context, e *domain.SessionEvent) error {
	if err := e.Validate(); err != nil {
		return fmt.Errorf("append session event: %w", err)
	}

	q := r.dbm.GetQuerier(ctx)

	if err := bbolt.Put(q, bucketSessionEvents, []byte(e.ID), internal.DomainToSessionEventMeta(e)); err != nil {
		return fmt.Errorf("append session event: %w", err)
	}

	if err := q.Bucket(bucketSessionEventsBySession).
		Put(sessionEventBySessionKey(e.SessionID, e.ID), nil); err != nil {
		return fmt.Errorf("append session event: session index: %w", err)
	}

	if err := q.Bucket(bucketSessionEventsByUser).
		Put(sessionEventByUserKey(e.UserID, e.ID), nil); err != nil {
		return fmt.Errorf("append session event: user index: %w", err)
	}

	return nil
}

// ListBySession returns all events for the given sessionID in storage order.
func (r *EventRepository) ListBySession(ctx context.Context, sessionID string) ([]*domain.SessionEvent, error) {
	events, err := r.listByPrefix(ctx, bucketSessionEventsBySession, sessionEventBySessionPrefix(sessionID))
	if err != nil {
		return nil, fmt.Errorf("list session events by session: %w", err)
	}

	return events, nil
}

// ListByUser returns events for userID with pagination. offset and limit
// follow standard slice semantics (limit ≤ 0 means return all remaining).
func (r *EventRepository) ListByUser(
	ctx context.Context,
	userID string,
	limit, offset int,
) ([]*domain.SessionEvent, error) {
	all, err := r.listByPrefix(ctx, bucketSessionEventsByUser, sessionEventByUserPrefix(userID))
	if err != nil {
		return nil, fmt.Errorf("list session events by user: %w", err)
	}

	if offset < 0 {
		offset = 0
	}

	if offset >= len(all) {
		return []*domain.SessionEvent{}, nil
	}

	end := len(all)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}

	return all[offset:end], nil
}

// listByPrefix scans the given index bucket for keys matching prefix and loads
// the corresponding events from the primary bucket.
func (r *EventRepository) listByPrefix(
	ctx context.Context,
	indexBucket string,
	prefix []byte,
) ([]*domain.SessionEvent, error) {
	var events []*domain.SessionEvent

	q := r.dbm.GetQuerier(ctx)

	err := q.Bucket(indexBucket).ForEach(func(k, _ []byte) error {
		if len(k) <= len(prefix) || !hasPrefix(k, prefix) {
			return nil
		}

		eventID := string(k[len(prefix):])

		m, err := bbolt.Get[internal.SessionEventMeta](q, bucketSessionEvents, []byte(eventID))
		if errors.Is(err, bbolt.ErrNotFound) {
			return nil // orphaned index entry — skip
		}
		if err != nil {
			return fmt.Errorf("get session event %s: %w", eventID, err)
		}

		events = append(events, internal.SessionEventMetaToDomain(m))

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan session events: %w", err)
	}

	return events, nil
}
