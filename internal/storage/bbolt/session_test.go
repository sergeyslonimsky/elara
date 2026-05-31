package bbolt_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	bboltadapter "github.com/sergeyslonimsky/elara/internal/storage/bbolt"
)

func newTestSession(id, userID string) *domain.Session {
	now := time.Now().UTC().Truncate(time.Millisecond)

	return &domain.Session{
		ID:         id,
		UserID:     userID,
		ClientType: domain.ClientTypeWeb,
		IP:         "127.0.0.1",
		UserAgent:  "test-agent",
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(24 * time.Hour),
	}
}

func assertSessionEqual(t *testing.T, want, got *domain.Session) {
	t.Helper()

	assert.Equal(t, want.ID, got.ID)
	assert.Equal(t, want.UserID, got.UserID)
	assert.Equal(t, want.ClientType, got.ClientType)
	assert.Equal(t, want.IP, got.IP)
	assert.Equal(t, want.UserAgent, got.UserAgent)
	assert.WithinDuration(t, want.CreatedAt, got.CreatedAt, time.Millisecond)
	assert.WithinDuration(t, want.LastSeenAt, got.LastSeenAt, time.Millisecond)
	assert.WithinDuration(t, want.ExpiresAt, got.ExpiresAt, time.Millisecond)
	assert.Equal(t, want.RevokedBy, got.RevokedBy)

	if want.RevokedAt == nil {
		assert.Nil(t, got.RevokedAt)
	} else {
		require.NotNil(t, got.RevokedAt)
		assert.WithinDuration(t, *want.RevokedAt, *got.RevokedAt, time.Millisecond)
	}
}

func TestSessionRepo_CreateGet_RoundTrip(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewSessionRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	s := newTestSession("sess-1", "user-a")
	require.NoError(t, repo.Create(ctx, s))

	got, err := repo.Get(ctx, "sess-1")
	require.NoError(t, err)

	assertSessionEqual(t, s, got)
}

func TestSessionRepo_Get_NotFound(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewSessionRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	_, err := repo.Get(ctx, "nonexistent-id")
	require.ErrorIs(t, err, domain.ErrSessionNotFound)
}

func TestSessionRepo_Update(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewSessionRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	s := newTestSession("sess-upd", "user-b")
	require.NoError(t, repo.Create(ctx, s))

	newLastSeen := s.LastSeenAt.Add(5 * time.Minute)
	s.LastSeenAt = newLastSeen
	require.NoError(t, repo.Update(ctx, s))

	got, err := repo.Get(ctx, "sess-upd")
	require.NoError(t, err)
	assert.WithinDuration(t, newLastSeen, got.LastSeenAt, time.Millisecond)
}

func TestSessionRepo_Update_NotFound(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewSessionRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	s := newTestSession("nonexistent-sess", "user-x")
	err := repo.Update(ctx, s)
	require.ErrorIs(t, err, domain.ErrSessionNotFound)
}

func TestSessionRepo_ListByUser(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewSessionRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	require.NoError(t, repo.Create(ctx, newTestSession("s-a1", "user-a")))
	require.NoError(t, repo.Create(ctx, newTestSession("s-a2", "user-a")))
	require.NoError(t, repo.Create(ctx, newTestSession("s-a3", "user-a")))
	require.NoError(t, repo.Create(ctx, newTestSession("s-b1", "user-b")))

	listA, err := repo.ListByUser(ctx, "user-a")
	require.NoError(t, err)
	assert.Len(t, listA, 3)

	listB, err := repo.ListByUser(ctx, "user-b")
	require.NoError(t, err)
	assert.Len(t, listB, 1)

	listUnknown, err := repo.ListByUser(ctx, "unknown-user")
	require.NoError(t, err)
	assert.Empty(t, listUnknown)
}

func TestSessionRepo_ListActiveByUser(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewSessionRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	s1 := newTestSession("s-act-1", "user-c")
	s2 := newTestSession("s-act-2", "user-c")
	s3 := newTestSession("s-act-3", "user-c")

	require.NoError(t, repo.Create(ctx, s1))
	require.NoError(t, repo.Create(ctx, s2))
	require.NoError(t, repo.Create(ctx, s3))

	// Revoke s3 via Update.
	s3.RevokedAt = new(time.Now().UTC())
	s3.RevokedBy = "admin"
	require.NoError(t, repo.Update(ctx, s3))

	active, err := repo.ListActiveByUser(ctx, "user-c")
	require.NoError(t, err)
	assert.Len(t, active, 2)

	for _, sess := range active {
		assert.Nil(t, sess.RevokedAt, "expected no revoked sessions in active list")
	}
}

func TestSessionRepo_RevokeAllForUser(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewSessionRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	s1 := newTestSession("s-rv-1", "user-d")
	s2 := newTestSession("s-rv-2", "user-d")
	s3 := newTestSession("s-rv-3", "user-d")

	require.NoError(t, repo.Create(ctx, s1))
	require.NoError(t, repo.Create(ctx, s2))
	require.NoError(t, repo.Create(ctx, s3))

	// Pre-revoke s3 with a different actor.
	s3.RevokedAt = new(time.Now().UTC().Add(-time.Minute))
	s3.RevokedBy = "original-revoker"
	require.NoError(t, repo.Update(ctx, s3))

	// Revoke all active sessions for user-d.
	count, err := repo.RevokeAllForUser(ctx, "user-d", "admin1")
	require.NoError(t, err)
	// Only 2 were active — s1 and s2.
	assert.Equal(t, 2, count)

	// All 3 sessions must now have RevokedAt set.
	for _, id := range []string{"s-rv-1", "s-rv-2", "s-rv-3"} {
		got, getErr := repo.Get(ctx, id)
		require.NoError(t, getErr)
		require.NotNil(t, got.RevokedAt, "session %s should be revoked", id)
	}

	// Previously-active sessions now show RevokedBy == "admin1".
	got1, err := repo.Get(ctx, "s-rv-1")
	require.NoError(t, err)
	assert.Equal(t, "admin1", got1.RevokedBy)

	got2, err := repo.Get(ctx, "s-rv-2")
	require.NoError(t, err)
	assert.Equal(t, "admin1", got2.RevokedBy)

	// The already-revoked session keeps its original RevokedBy (implementation skips it).
	got3, err := repo.Get(ctx, "s-rv-3")
	require.NoError(t, err)
	assert.Equal(t, "original-revoker", got3.RevokedBy)
}

func TestSessionRepo_Concurrent_CreateList(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewSessionRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	const n = 20

	var wg sync.WaitGroup

	wg.Add(n)

	for i := range n {
		go func(i int) {
			defer wg.Done()

			s := newTestSession(fmt.Sprintf("concurrent-sess-%d", i), "concurrent-user")
			_ = repo.Create(ctx, s)
		}(i)
	}

	wg.Wait()

	all, err := repo.ListByUser(ctx, "concurrent-user")
	require.NoError(t, err)
	assert.Len(t, all, n)
}
