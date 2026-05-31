package bbolt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// SessionEventRepo stores and retrieves session events in bbolt.
// It is append-only: no Update or Delete methods.
type SessionEventRepo struct {
	manager *Manager
}

// NewSessionEventRepo creates a new SessionEventRepo backed by the given Manager.
func NewSessionEventRepo(manager *Manager) *SessionEventRepo {
	return &SessionEventRepo{manager: manager}
}

// Append validates and persists a new SessionEvent. Returns a validation error
// when the event is incomplete.
func (r *SessionEventRepo) Append(ctx context.Context, e *domain.SessionEvent) error {
	if err := e.Validate(); err != nil {
		return fmt.Errorf("append session event: %w", err)
	}

	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketSessionEvents))
		// ...

		data, err := json.Marshal(domainToSessionEventMeta(e))
		if err != nil {
			return fmt.Errorf("marshal session event: %w", err)
		}

		if err = b.Put([]byte(e.ID), data); err != nil {
			return fmt.Errorf("put session event: %w", err)
		}

		idxSession := tx.Bucket([]byte(bucketSessionEventsBySession))
		if err = idxSession.Put(sessionEventBySessionKey(e.SessionID, e.ID), nil); err != nil {
			return fmt.Errorf("put session event session index: %w", err)
		}

		idxUser := tx.Bucket([]byte(bucketSessionEventsByUser))

		return idxUser.Put(sessionEventByUserKey(e.UserID, e.ID), nil)
	})
	if err != nil {
		return fmt.Errorf("append session event: %w", err)
	}

	return nil
}

// ListBySession returns all events for the given sessionID in storage order.
func (r *SessionEventRepo) ListBySession(ctx context.Context, sessionID string) ([]*domain.SessionEvent, error) {
	events, err := r.listByPrefix(ctx, []byte(bucketSessionEventsBySession), sessionEventBySessionPrefix(sessionID))
	if err != nil {
		return nil, fmt.Errorf("list session events by session: %w", err)
	}

	return events, nil
}

// ListByUser returns events for userID with pagination. offset and limit follow
// standard slice semantics (limit ≤ 0 means return all remaining).
func (r *SessionEventRepo) ListByUser(
	ctx context.Context,
	userID string,
	limit, offset int,
) ([]*domain.SessionEvent, error) {
	all, err := r.listByPrefix(ctx, []byte(bucketSessionEventsByUser), sessionEventByUserPrefix(userID))
	if err != nil {
		return nil, fmt.Errorf("list session events by user: %w", err)
	}
	// ...

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
func (r *SessionEventRepo) listByPrefix(
	ctx context.Context,
	indexBucket, prefix []byte,
) ([]*domain.SessionEvent, error) {
	var events []*domain.SessionEvent

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
		idx := tx.Bucket(indexBucket)
		b := tx.Bucket([]byte(bucketSessionEvents))
		// ...

		return idx.ForEach(func(k, _ []byte) error {
			if len(k) <= len(prefix) || !bytes.Equal(k[:len(prefix)], prefix) {
				return nil
			}

			// The event ID follows the prefix separator.
			eventID := string(k[len(prefix):])
			data := b.Get([]byte(eventID))

			if data == nil {
				return nil // orphaned index entry — skip
			}

			m, err := sessionEventMetaFromBytes(data)
			if err != nil {
				return err
			}

			events = append(events, sessionEventMetaToDomain(m))

			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return events, nil
}

func sessionEventBySessionKey(sessionID, eventID string) []byte {
	return append(sessionEventBySessionPrefix(sessionID), []byte(eventID)...)
}

func sessionEventBySessionPrefix(sessionID string) []byte {
	return []byte(sessionID + string(keySep))
}

func sessionEventByUserKey(userID, eventID string) []byte {
	return append(sessionEventByUserPrefix(userID), []byte(eventID)...)
}

func sessionEventByUserPrefix(userID string) []byte {
	return []byte(userID + string(keySep))
}
