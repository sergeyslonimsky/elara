// Package session is the bbolt-backed repository for domain.Session and
// domain.SessionEvent (server-side user sessions + their audit log).
//
// Repository contract:
//   - Pure CRUD (Get, Create, Update) on Repository is "dumb": callers
//     prepare every field, the repo writes verbatim.
//   - Create / Delete keep the `sessions_by_user` secondary index in sync.
//     Atomicity of the multi-bucket write is the caller's responsibility:
//     wrap in Manager.WithTx if the partial state matters.
//   - RevokeAllForUser is a load-then-write fan-out across many sessions —
//     caller MUST wrap in Manager.WithTx to keep the bulk update atomic.
//   - EventRepository (in event_repository.go) is the append-only audit
//     log; same contract: caller owns atomicity across primary bucket and
//     two indexes.
//   - All methods read/write through the pkg/bbolt querier, which joins
//     the surrounding transaction when present in ctx.
package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/storage/internal"
	"github.com/sergeyslonimsky/elara/pkg/bbolt"
)

type Repository struct {
	dbm bbolt.Manager
}

func NewRepository(dbm bbolt.Manager) *Repository {
	return &Repository{dbm: dbm}
}

// Get returns the Session identified by id.
// Returns storage.ErrResourceNotFound if missing.
func (r *Repository) Get(ctx context.Context, id string) (*domain.Session, error) {
	m, err := bbolt.Get[internal.SessionMeta](r.dbm.GetQuerier(ctx), bucketSessions, []byte(id))
	if errors.Is(err, bbolt.ErrNotFound) {
		return nil, fmt.Errorf("session %s: %w", id, storage.ErrResourceNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	return internal.SessionMetaToDomain(m), nil
}

// Create stores a new Session and writes a secondary index entry by user ID.
//
// Atomicity across the two buckets is the caller's responsibility.
func (r *Repository) Create(ctx context.Context, s *domain.Session) error {
	q := r.dbm.GetQuerier(ctx)

	if err := bbolt.Put(q, bucketSessions, []byte(s.ID), internal.DomainToSessionMeta(s)); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	if err := q.Bucket(bucketSessionsByUser).Put(sessionByUserKey(s.UserID, s.ID), nil); err != nil {
		return fmt.Errorf("create session: index: %w", err)
	}

	return nil
}

// Update replaces a Session in place. Returns storage.ErrResourceNotFound if missing.
//
// Atomicity (exists-check + put) is the caller's responsibility.
func (r *Repository) Update(ctx context.Context, s *domain.Session) error {
	q := r.dbm.GetQuerier(ctx)

	if !bbolt.Exists(q, bucketSessions, []byte(s.ID)) {
		return fmt.Errorf("session %s: %w", s.ID, storage.ErrResourceNotFound)
	}

	if err := bbolt.Put(q, bucketSessions, []byte(s.ID), internal.DomainToSessionMeta(s)); err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	return nil
}

// ListByUser returns all sessions belonging to userID.
func (r *Repository) ListByUser(ctx context.Context, userID string) ([]*domain.Session, error) {
	sessions, err := r.listByUserPrefix(ctx, userID, false)
	if err != nil {
		return nil, fmt.Errorf("list sessions by user: %w", err)
	}

	return sessions, nil
}

// ListActiveByUser returns sessions for userID that have not been revoked.
func (r *Repository) ListActiveByUser(ctx context.Context, userID string) ([]*domain.Session, error) {
	sessions, err := r.listByUserPrefix(ctx, userID, true)
	if err != nil {
		return nil, fmt.Errorf("list active sessions by user: %w", err)
	}

	return sessions, nil
}

// RevokeAllForUser marks every active session for userID as revoked.
// Returns the number of sessions that were changed.
//
// Does NOT append SessionEvent records — that is the service layer's responsibility.
//
// The bulk load+write is NOT atomic without a caller-side Manager.WithTx;
// usecases that need a consistent snapshot MUST wrap this call.
func (r *Repository) RevokeAllForUser(ctx context.Context, userID, revokedBy string) (int, error) {
	q := r.dbm.GetQuerier(ctx)
	prefix := sessionByUserPrefix(userID)
	now := time.Now()

	ids, err := collectSessionIDsForUser(q, prefix)
	if err != nil {
		return 0, fmt.Errorf("revoke all sessions for user: %w", err)
	}

	var count int

	for _, id := range ids {
		revoked, err := revokeSessionIfActive(q, id, revokedBy, now)
		if err != nil {
			return 0, fmt.Errorf("revoke all sessions for user: %w", err)
		}

		if revoked {
			count++
		}
	}

	return count, nil
}

// listByUserPrefix scans the sessions_by_user index for all sessions owned by
// userID and fetches each from the primary bucket. When activeOnly is true,
// revoked sessions are excluded.
func (r *Repository) listByUserPrefix(
	ctx context.Context,
	userID string,
	activeOnly bool,
) ([]*domain.Session, error) {
	var sessions []*domain.Session

	q := r.dbm.GetQuerier(ctx)
	idx := q.Bucket(bucketSessionsByUser)
	prefix := sessionByUserPrefix(userID)

	err := idx.ForEach(func(k, _ []byte) error {
		if len(k) <= len(prefix) || !hasPrefix(k, prefix) {
			return nil
		}

		sessionID := string(k[len(prefix):])

		m, err := bbolt.Get[internal.SessionMeta](q, bucketSessions, []byte(sessionID))
		if errors.Is(err, bbolt.ErrNotFound) {
			return nil // orphaned index entry — skip
		}
		if err != nil {
			return fmt.Errorf("get session %s: %w", sessionID, err)
		}

		if activeOnly && m.RevokedAt != nil {
			return nil
		}

		sessions = append(sessions, internal.SessionMetaToDomain(m))

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan sessions by user: %w", err)
	}

	return sessions, nil
}

// collectSessionIDsForUser scans the sessions_by_user index and returns all
// session IDs whose key starts with prefix.
func collectSessionIDsForUser(q bbolt.Querier, prefix []byte) ([]string, error) {
	var ids []string

	err := q.Bucket(bucketSessionsByUser).ForEach(func(k, _ []byte) error {
		if len(k) > len(prefix) && hasPrefix(k, prefix) {
			ids = append(ids, string(k[len(prefix):]))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan sessions by user index: %w", err)
	}

	return ids, nil
}

// revokeSessionIfActive loads the session, marks it revoked if it is not
// already, and writes it back. Returns true when a write occurred.
func revokeSessionIfActive(q bbolt.Querier, id, revokedBy string, now time.Time) (bool, error) {
	m, err := bbolt.Get[internal.SessionMeta](q, bucketSessions, []byte(id))
	if errors.Is(err, bbolt.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get session %s: %w", id, err)
	}

	if m.RevokedAt != nil {
		return false, nil
	}

	m.RevokedAt = &now
	m.RevokedBy = revokedBy

	if err := bbolt.Put(q, bucketSessions, []byte(id), m); err != nil {
		return false, fmt.Errorf("put revoked session: %w", err)
	}

	return true, nil
}
