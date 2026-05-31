package bbolt_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	bboltadapter "github.com/sergeyslonimsky/elara/internal/storage/bbolt"
)

func newTestSessionEvent(id, sessionID, userID string) *domain.SessionEvent {
	return &domain.SessionEvent{
		ID:        id,
		SessionID: sessionID,
		UserID:    userID,
		Type:      domain.SessionEventCreated,
		IP:        "127.0.0.1",
		UserAgent: "test-agent",
		Timestamp: time.Now().UTC().Truncate(time.Millisecond),
	}
}

func TestSessionEventRepo_Append_ListBySession(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewSessionEventRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	require.NoError(t, repo.Append(ctx, newTestSessionEvent("ev-s1-1", "sess-1", "user-a")))
	require.NoError(t, repo.Append(ctx, newTestSessionEvent("ev-s1-2", "sess-1", "user-a")))
	require.NoError(t, repo.Append(ctx, newTestSessionEvent("ev-s1-3", "sess-1", "user-a")))
	require.NoError(t, repo.Append(ctx, newTestSessionEvent("ev-s2-1", "sess-2", "user-a")))
	require.NoError(t, repo.Append(ctx, newTestSessionEvent("ev-s2-2", "sess-2", "user-a")))

	eventsS1, err := repo.ListBySession(ctx, "sess-1")
	require.NoError(t, err)
	assert.Len(t, eventsS1, 3)

	eventsS2, err := repo.ListBySession(ctx, "sess-2")
	require.NoError(t, err)
	assert.Len(t, eventsS2, 2)
}

func TestSessionEventRepo_ListByUser_Pagination(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewSessionEventRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	for i := range 5 {
		ev := newTestSessionEvent(
			// Use deterministic IDs so ordering is predictable in bbolt (lexicographic).
			// Prefix with zero-padded index to ensure consistent scan order.
			"paginate-ev-0"+string(rune('0'+i)),
			"paginate-sess",
			"user-paginate",
		)
		require.NoError(t, repo.Append(ctx, ev))
	}

	// limit=2, offset=0 → first 2.
	page0, err := repo.ListByUser(ctx, "user-paginate", 2, 0)
	require.NoError(t, err)
	assert.Len(t, page0, 2)

	// limit=2, offset=2 → next 2.
	page1, err := repo.ListByUser(ctx, "user-paginate", 2, 2)
	require.NoError(t, err)
	assert.Len(t, page1, 2)

	// limit=2, offset=4 → last 1.
	page2, err := repo.ListByUser(ctx, "user-paginate", 2, 4)
	require.NoError(t, err)
	assert.Len(t, page2, 1)

	// offset beyond length → empty.
	page3, err := repo.ListByUser(ctx, "user-paginate", 2, 10)
	require.NoError(t, err)
	assert.Empty(t, page3)

	// Union of all pages covers all 5 events.
	all, err := repo.ListByUser(ctx, "user-paginate", 0, 0)
	require.NoError(t, err)
	assert.Len(t, all, 5)
}

func TestSessionEventRepo_Append_InvalidEvent(t *testing.T) {
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
				ID:        "ev-invalid-type",
				SessionID: "sess-x",
				UserID:    "user-x",
				Type:      "",
				Timestamp: now,
			},
			wantErr: "type is required",
		},
		{
			name: "missing SessionID",
			event: &domain.SessionEvent{
				ID:        "ev-invalid-session",
				SessionID: "",
				UserID:    "user-x",
				Type:      domain.SessionEventCreated,
				Timestamp: now,
			},
			wantErr: "session_id is required",
		},
		{
			name: "missing UserID",
			event: &domain.SessionEvent{
				ID:        "ev-invalid-user",
				SessionID: "sess-x",
				UserID:    "",
				Type:      domain.SessionEventCreated,
				Timestamp: now,
			},
			wantErr: "user_id is required",
		},
		{
			name: "zero Timestamp",
			event: &domain.SessionEvent{
				ID:        "ev-invalid-ts",
				SessionID: "sess-x",
				UserID:    "user-x",
				Type:      domain.SessionEventCreated,
				Timestamp: time.Time{},
			},
			wantErr: "timestamp is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := newTestStore(t)
			repo := bboltadapter.NewSessionEventRepo(bboltadapter.NewManager(store.DB()))
			ctx := t.Context()

			err := repo.Append(ctx, tt.event)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestSessionEventRepo_ListBySession_Unknown(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewSessionEventRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	events, err := repo.ListBySession(ctx, "unknown-session-id")
	require.NoError(t, err)
	assert.Empty(t, events)
}
