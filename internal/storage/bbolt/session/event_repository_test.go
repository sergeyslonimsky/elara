package session_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	sessionrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/session"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

func newEventRepo(t *testing.T) *sessionrepo.EventRepository {
	t.Helper()

	path := filepath.Join(t.TempDir(), "elara.db")
	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	mgr := pkgbbolt.NewManager(store.DB())

	return sessionrepo.NewEventRepository(mgr)
}

func newEvent(
	t *testing.T,
	id, sessionID, userID string,
	overrides ...func(*domain.SessionEvent),
) *domain.SessionEvent {
	t.Helper()

	e := &domain.SessionEvent{
		ID:        id,
		SessionID: sessionID,
		UserID:    userID,
		Type:      domain.SessionEventCreated,
		IP:        "127.0.0.1",
		UserAgent: "test-agent",
		Timestamp: time.Now().UTC().Truncate(time.Millisecond),
	}
	for _, o := range overrides {
		o(e)
	}

	return e
}

func TestEventRepository_Append(t *testing.T) {
	t.Parallel()

	t.Run("writes primary + both indexes (findable by session and user)", func(t *testing.T) {
		t.Parallel()
		repo := newEventRepo(t)
		ctx := t.Context()

		ev := newEvent(t, "ev-1", "sess-1", "user-a")
		require.NoError(t, repo.Append(ctx, ev))

		bySession, err := repo.ListBySession(ctx, "sess-1")
		require.NoError(t, err)
		require.Len(t, bySession, 1)
		assert.Equal(t, "ev-1", bySession[0].ID)

		byUser, err := repo.ListByUser(ctx, "user-a", 0, 0)
		require.NoError(t, err)
		require.Len(t, byUser, 1)
		assert.Equal(t, "ev-1", byUser[0].ID)
	})

	t.Run("validation errors", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		tests := []struct {
			name    string
			event   *domain.SessionEvent
			wantErr string
		}{
			{
				name: "missing Type",
				event: &domain.SessionEvent{
					ID: "e1", SessionID: "s", UserID: "u", Timestamp: now,
				},
				wantErr: "type is required",
			},
			{
				name: "missing SessionID",
				event: &domain.SessionEvent{
					ID: "e2", UserID: "u", Type: domain.SessionEventCreated, Timestamp: now,
				},
				wantErr: "session_id is required",
			},
			{
				name: "missing UserID",
				event: &domain.SessionEvent{
					ID: "e3", SessionID: "s", Type: domain.SessionEventCreated, Timestamp: now,
				},
				wantErr: "user_id is required",
			},
			{
				name: "zero Timestamp",
				event: &domain.SessionEvent{
					ID: "e4", SessionID: "s", UserID: "u", Type: domain.SessionEventCreated,
				},
				wantErr: "timestamp is required",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				repo := newEventRepo(t)

				err := repo.Append(t.Context(), tt.event)
				require.ErrorContains(t, err, tt.wantErr)
			})
		}
	})
}

func TestEventRepository_ListBySession(t *testing.T) {
	t.Parallel()

	t.Run("returns events scoped to session", func(t *testing.T) {
		t.Parallel()
		repo := newEventRepo(t)
		ctx := t.Context()

		require.NoError(t, repo.Append(ctx, newEvent(t, "ev-s1-1", "sess-1", "user-a")))
		require.NoError(t, repo.Append(ctx, newEvent(t, "ev-s1-2", "sess-1", "user-a")))
		require.NoError(t, repo.Append(ctx, newEvent(t, "ev-s1-3", "sess-1", "user-a")))
		require.NoError(t, repo.Append(ctx, newEvent(t, "ev-s2-1", "sess-2", "user-a")))

		gotS1, err := repo.ListBySession(ctx, "sess-1")
		require.NoError(t, err)
		assert.Len(t, gotS1, 3)

		gotS2, err := repo.ListBySession(ctx, "sess-2")
		require.NoError(t, err)
		assert.Len(t, gotS2, 1)
		assert.Equal(t, "ev-s2-1", gotS2[0].ID)
	})

	t.Run("unknown session returns empty", func(t *testing.T) {
		t.Parallel()
		repo := newEventRepo(t)

		got, err := repo.ListBySession(t.Context(), "unknown")
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestEventRepository_ListByUser(t *testing.T) {
	t.Parallel()

	t.Run("isolated per user", func(t *testing.T) {
		t.Parallel()
		repo := newEventRepo(t)
		ctx := t.Context()

		require.NoError(t, repo.Append(ctx, newEvent(t, "ev-u1-a", "sess-a", "user-1")))
		require.NoError(t, repo.Append(ctx, newEvent(t, "ev-u1-b", "sess-b", "user-1")))
		require.NoError(t, repo.Append(ctx, newEvent(t, "ev-u2-a", "sess-c", "user-2")))

		gotU1, err := repo.ListByUser(ctx, "user-1", 0, 0)
		require.NoError(t, err)
		assert.Len(t, gotU1, 2)

		gotU2, err := repo.ListByUser(ctx, "user-2", 0, 0)
		require.NoError(t, err)
		assert.Len(t, gotU2, 1)
		assert.Equal(t, "ev-u2-a", gotU2[0].ID)
	})

	t.Run("pagination slices the result", func(t *testing.T) {
		t.Parallel()
		repo := newEventRepo(t)
		ctx := t.Context()

		for _, id := range []string{"p-0", "p-1", "p-2", "p-3", "p-4"} {
			require.NoError(t, repo.Append(ctx, newEvent(t, id, "p-sess", "p-user")))
		}

		page0, err := repo.ListByUser(ctx, "p-user", 2, 0)
		require.NoError(t, err)
		assert.Len(t, page0, 2)

		page1, err := repo.ListByUser(ctx, "p-user", 2, 2)
		require.NoError(t, err)
		assert.Len(t, page1, 2)

		page2, err := repo.ListByUser(ctx, "p-user", 2, 4)
		require.NoError(t, err)
		assert.Len(t, page2, 1)

		// Offset beyond length → empty.
		page3, err := repo.ListByUser(ctx, "p-user", 2, 10)
		require.NoError(t, err)
		assert.Empty(t, page3)

		// limit <= 0 → return all remaining.
		all, err := repo.ListByUser(ctx, "p-user", 0, 0)
		require.NoError(t, err)
		assert.Len(t, all, 5)

		// Negative offset is treated as 0.
		negOff, err := repo.ListByUser(ctx, "p-user", 0, -3)
		require.NoError(t, err)
		assert.Len(t, negOff, 5)
	})

	t.Run("unknown user returns empty", func(t *testing.T) {
		t.Parallel()
		repo := newEventRepo(t)

		got, err := repo.ListByUser(t.Context(), "unknown", 0, 0)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}
