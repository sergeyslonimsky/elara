package session_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	sessionrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/session"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

func newRepo(t *testing.T) (*sessionrepo.Repository, pkgbbolt.Manager) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "elara.db")
	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	mgr := pkgbbolt.NewManager(store.DB())

	return sessionrepo.NewRepository(mgr), mgr
}

func newSession(t *testing.T, id, userID string, overrides ...func(*domain.Session)) *domain.Session {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Millisecond)
	s := &domain.Session{
		ID:         id,
		UserID:     userID,
		ClientType: domain.ClientTypeWeb,
		IP:         "127.0.0.1",
		UserAgent:  "test-agent",
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(24 * time.Hour),
	}
	for _, o := range overrides {
		o(s)
	}

	return s
}

func TestRepository_Get(t *testing.T) {
	t.Parallel()

	t.Run("success returns stored session", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		s := newSession(t, "sess-1", "user-a")
		require.NoError(t, repo.Create(ctx, s))

		got, err := repo.Get(ctx, "sess-1")
		require.NoError(t, err)
		assert.Equal(t, s.ID, got.ID)
		assert.Equal(t, s.UserID, got.UserID)
		assert.Equal(t, s.ClientType, got.ClientType)
		assert.Equal(t, s.IP, got.IP)
		assert.Equal(t, s.UserAgent, got.UserAgent)
		assert.WithinDuration(t, s.CreatedAt, got.CreatedAt, time.Millisecond)
		assert.WithinDuration(t, s.LastSeenAt, got.LastSeenAt, time.Millisecond)
		assert.WithinDuration(t, s.ExpiresAt, got.ExpiresAt, time.Millisecond)
		assert.Nil(t, got.RevokedAt)
	})

	t.Run("missing returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		_, err := repo.Get(t.Context(), "nope")
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_Create(t *testing.T) {
	t.Parallel()

	t.Run("success writes primary + user index", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		s := newSession(t, "sess-c1", "user-a")
		require.NoError(t, repo.Create(ctx, s))

		// Primary bucket: Get works.
		got, err := repo.Get(ctx, s.ID)
		require.NoError(t, err)
		assert.Equal(t, s.ID, got.ID)

		// Secondary index: ListByUser finds it.
		listed, err := repo.ListByUser(ctx, "user-a")
		require.NoError(t, err)
		require.Len(t, listed, 1)
		assert.Equal(t, s.ID, listed[0].ID)
	})
}

func TestRepository_Update(t *testing.T) {
	t.Parallel()

	t.Run("success persists field changes", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		s := newSession(t, "sess-u1", "user-a")
		require.NoError(t, repo.Create(ctx, s))

		newLastSeen := s.LastSeenAt.Add(5 * time.Minute)
		s.LastSeenAt = newLastSeen
		require.NoError(t, repo.Update(ctx, s))

		got, err := repo.Get(ctx, s.ID)
		require.NoError(t, err)
		assert.WithinDuration(t, newLastSeen, got.LastSeenAt, time.Millisecond)
	})

	t.Run("missing returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		s := newSession(t, "nonexistent", "user-x")
		err := repo.Update(t.Context(), s)
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_ListByUser(t *testing.T) {
	t.Parallel()

	t.Run("returns all sessions for a user (active + revoked)", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		require.NoError(t, repo.Create(ctx, newSession(t, "s-a1", "user-a")))
		require.NoError(t, repo.Create(ctx, newSession(t, "s-a2", "user-a")))

		// Revoke one — should still appear in ListByUser.
		revoked := newSession(t, "s-a3", "user-a")
		require.NoError(t, repo.Create(ctx, revoked))
		now := time.Now().UTC()
		revoked.RevokedAt = &now
		revoked.RevokedBy = "admin"
		require.NoError(t, repo.Update(ctx, revoked))

		// Other user.
		require.NoError(t, repo.Create(ctx, newSession(t, "s-b1", "user-b")))

		listA, err := repo.ListByUser(ctx, "user-a")
		require.NoError(t, err)
		assert.Len(t, listA, 3)

		listB, err := repo.ListByUser(ctx, "user-b")
		require.NoError(t, err)
		assert.Len(t, listB, 1)
		assert.Equal(t, "s-b1", listB[0].ID)
	})

	t.Run("unknown user returns empty", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		got, err := repo.ListByUser(t.Context(), "unknown")
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestRepository_ListActiveByUser(t *testing.T) {
	t.Parallel()

	t.Run("filters out revoked sessions", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		s1 := newSession(t, "s-act-1", "user-c")
		s2 := newSession(t, "s-act-2", "user-c")
		s3 := newSession(t, "s-act-3", "user-c")
		require.NoError(t, repo.Create(ctx, s1))
		require.NoError(t, repo.Create(ctx, s2))
		require.NoError(t, repo.Create(ctx, s3))

		// Revoke s3.
		now := time.Now().UTC()
		s3.RevokedAt = &now
		s3.RevokedBy = "admin"
		require.NoError(t, repo.Update(ctx, s3))

		active, err := repo.ListActiveByUser(ctx, "user-c")
		require.NoError(t, err)
		assert.Len(t, active, 2)
		for _, sess := range active {
			assert.Nil(t, sess.RevokedAt)
		}
	})

	t.Run("isolated per user", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		require.NoError(t, repo.Create(ctx, newSession(t, "s-iso-a", "user-a")))
		require.NoError(t, repo.Create(ctx, newSession(t, "s-iso-b", "user-b")))

		got, err := repo.ListActiveByUser(ctx, "user-a")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "s-iso-a", got[0].ID)
	})
}

func TestRepository_RevokeAllForUser(t *testing.T) {
	t.Parallel()

	t.Run("revokes only active sessions and is idempotent", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		s1 := newSession(t, "s-rv-1", "user-d")
		s2 := newSession(t, "s-rv-2", "user-d")
		s3 := newSession(t, "s-rv-3", "user-d")
		require.NoError(t, repo.Create(ctx, s1))
		require.NoError(t, repo.Create(ctx, s2))
		require.NoError(t, repo.Create(ctx, s3))

		// Pre-revoke s3 with a different actor.
		preRevoke := time.Now().UTC().Add(-time.Minute)
		s3.RevokedAt = &preRevoke
		s3.RevokedBy = "original-revoker"
		require.NoError(t, repo.Update(ctx, s3))

		// First call revokes 2 (s1, s2).
		count, err := repo.RevokeAllForUser(ctx, "user-d", "admin1")
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		// All three are now revoked.
		for _, id := range []string{"s-rv-1", "s-rv-2", "s-rv-3"} {
			got, getErr := repo.Get(ctx, id)
			require.NoError(t, getErr)
			require.NotNil(t, got.RevokedAt, "session %s should be revoked", id)
		}

		// Newly-revoked sessions show admin1 as actor.
		got1, err := repo.Get(ctx, "s-rv-1")
		require.NoError(t, err)
		assert.Equal(t, "admin1", got1.RevokedBy)

		// Pre-existing revocation is preserved.
		got3, err := repo.Get(ctx, "s-rv-3")
		require.NoError(t, err)
		assert.Equal(t, "original-revoker", got3.RevokedBy)

		// Idempotent: second call revokes 0.
		count2, err := repo.RevokeAllForUser(ctx, "user-d", "admin2")
		require.NoError(t, err)
		assert.Equal(t, 0, count2)
	})

	t.Run("leaves other users untouched", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		require.NoError(t, repo.Create(ctx, newSession(t, "mine", "user-d")))
		require.NoError(t, repo.Create(ctx, newSession(t, "theirs", "user-e")))

		count, err := repo.RevokeAllForUser(ctx, "user-d", "admin1")
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		other, err := repo.Get(ctx, "theirs")
		require.NoError(t, err)
		assert.Nil(t, other.RevokedAt)
	})

	t.Run("no sessions returns zero", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		count, err := repo.RevokeAllForUser(t.Context(), "user-empty", "admin")
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestRepository_WithTx_RollbackDiscardsWrites(t *testing.T) {
	t.Parallel()

	repo, mgr := newRepo(t)
	ctx := t.Context()

	a := newSession(t, "tx-a", "user-tx")
	b := newSession(t, "tx-b", "user-tx")

	err := mgr.WithTx(ctx, func(ctx context.Context) error {
		if err := repo.Create(ctx, a); err != nil {
			return err
		}
		if err := repo.Create(ctx, b); err != nil {
			return err
		}

		return assert.AnError
	})
	require.ErrorIs(t, err, assert.AnError)

	got, err := repo.ListByUser(ctx, "user-tx")
	require.NoError(t, err)
	assert.Empty(t, got)
}
