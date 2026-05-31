package bbolt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// SessionRepo stores and retrieves sessions in bbolt.
type SessionRepo struct {
	manager *Manager
}

// NewSessionRepo creates a new SessionRepo backed by the given Manager.
func NewSessionRepo(manager *Manager) *SessionRepo {
	return &SessionRepo{manager: manager}
}

// Get returns the Session identified by id.
// Returns domain.ErrSessionNotFound if missing.
func (r *SessionRepo) Get(ctx context.Context, id string) (*domain.Session, error) {
	var session *domain.Session

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketSessions))
		// ...
		data := b.Get([]byte(id))

		if data == nil {
			return domain.ErrSessionNotFound
		}

		m, err := sessionMetaFromBytes(data)
		if err != nil {
			return err
		}

		session = sessionMetaToDomain(m)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	return session, nil
}

// Create stores a new Session and writes a secondary index entry by user ID.
func (r *SessionRepo) Create(ctx context.Context, s *domain.Session) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketSessions))
		// ...

		data, err := json.Marshal(domainToSessionMeta(s))
		if err != nil {
			return fmt.Errorf("marshal session: %w", err)
		}

		if err = b.Put([]byte(s.ID), data); err != nil {
			return fmt.Errorf("put session: %w", err)
		}

		idx := tx.Bucket([]byte(bucketSessionsByUser))

		return idx.Put(sessionByUserKey(s.UserID, s.ID), nil)
	})
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	return nil
}

// Update replaces a Session in place. Returns domain.ErrSessionNotFound if missing.
func (r *SessionRepo) Update(ctx context.Context, s *domain.Session) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketSessions))
		key := []byte(s.ID)

		if b.Get(key) == nil {
			return domain.ErrSessionNotFound
		}

		data, err := json.Marshal(domainToSessionMeta(s))
		if err != nil {
			return fmt.Errorf("marshal session: %w", err)
		}

		return b.Put(key, data)
	})
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	return nil
}

// ListByUser returns all sessions belonging to userID.
func (r *SessionRepo) ListByUser(ctx context.Context, userID string) ([]*domain.Session, error) {
	sessions, err := r.listByUserPrefix(ctx, userID, false)
	if err != nil {
		return nil, fmt.Errorf("list sessions by user: %w", err)
	}

	return sessions, nil
}

// ListActiveByUser returns sessions for userID that have not been revoked.
func (r *SessionRepo) ListActiveByUser(ctx context.Context, userID string) ([]*domain.Session, error) {
	sessions, err := r.listByUserPrefix(ctx, userID, true)
	if err != nil {
		return nil, fmt.Errorf("list active sessions by user: %w", err)
	}

	return sessions, nil
}

// RevokeAllForUser atomically marks every active session for userID as revoked.
// Returns the number of sessions that were changed.
// Does NOT append SessionEvent records — that is the service layer's responsibility.
func (r *SessionRepo) RevokeAllForUser(ctx context.Context, userID, revokedBy string) (int, error) {
	var count int

	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		idx := tx.Bucket([]byte(bucketSessionsByUser))
		b := tx.Bucket([]byte(bucketSessions))
		// ...
		prefix := sessionByUserPrefix(userID)
		now := time.Now()

		ids, err := collectSessionIDsForUser(idx, prefix)
		if err != nil {
			return err
		}

		for _, id := range ids {
			revoked, err := revokeSessionIfActive(b, id, revokedBy, now)
			if err != nil {
				return err
			}

			if revoked {
				count++
			}
		}

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("revoke all sessions for user: %w", err)
	}

	return count, nil
}

// collectSessionIDsForUser scans the sessions_by_user index bucket and returns
// all session IDs whose key starts with prefix.
func collectSessionIDsForUser(idx *bolt.Bucket, prefix []byte) ([]string, error) {
	var ids []string

	if err := idx.ForEach(func(k, _ []byte) error {
		if len(k) > len(prefix) && bytes.Equal(k[:len(prefix)], prefix) {
			ids = append(ids, string(k[len(prefix):]))
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan sessions by user index: %w", err)
	}

	return ids, nil
}

// revokeSessionIfActive loads session id from b, marks it revoked if it is not
// already, and writes it back. Returns true when a write occurred.
func revokeSessionIfActive(b *bolt.Bucket, id, revokedBy string, now time.Time) (bool, error) {
	data := b.Get([]byte(id))
	if data == nil {
		return false, nil
	}

	m, err := sessionMetaFromBytes(data)
	if err != nil {
		return false, err
	}

	if m.RevokedAt != nil {
		return false, nil // already revoked
	}

	m.RevokedAt = &now
	m.RevokedBy = revokedBy

	newData, err := json.Marshal(m)
	if err != nil {
		return false, fmt.Errorf("marshal session: %w", err)
	}

	if err = b.Put([]byte(id), newData); err != nil {
		return false, fmt.Errorf("put revoked session: %w", err)
	}

	return true, nil
}

// listByUserPrefix scans the sessions_by_user index for all sessions owned by
// userID and fetches each from the primary bucket. When activeOnly is true,
// revoked sessions are excluded.
func (r *SessionRepo) listByUserPrefix(ctx context.Context, userID string, activeOnly bool) ([]*domain.Session, error) {
	var sessions []*domain.Session

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
		idx := tx.Bucket([]byte(bucketSessionsByUser))
		b := tx.Bucket([]byte(bucketSessions))
		prefix := sessionByUserPrefix(userID)
		// ...

		return idx.ForEach(func(k, _ []byte) error {
			if len(k) <= len(prefix) || !bytes.Equal(k[:len(prefix)], prefix) {
				return nil
			}

			sessionID := string(k[len(prefix):])
			data := b.Get([]byte(sessionID))

			if data == nil {
				return nil // orphaned index entry — skip
			}

			m, err := sessionMetaFromBytes(data)
			if err != nil {
				return err
			}

			if activeOnly && m.RevokedAt != nil {
				return nil
			}

			sessions = append(sessions, sessionMetaToDomain(m))

			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return sessions, nil
}

// sessionByUserKey returns the composite key for the sessions_by_user index.
// Format: <userID> + keySep + <sessionID>.
func sessionByUserKey(userID, sessionID string) []byte {
	return append(sessionByUserPrefix(userID), []byte(sessionID)...)
}

func sessionByUserPrefix(userID string) []byte {
	return []byte(userID + string(keySep))
}
