package bbolt_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bboltadapter "github.com/sergeyslonimsky/elara/internal/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

func makeSnap(id string, disconnectedAt time.Time) *domain.Client {
	return &domain.Client{
		ID:             id,
		PeerAddress:    "10.0.0.1:1234",
		ClientName:     "svc",
		ConnectedAt:    disconnectedAt.Add(-5 * time.Minute),
		DisconnectedAt: new(disconnectedAt),
		LastActivityAt: disconnectedAt,
		ActiveWatches:  0,
		RequestCounts:  map[string]int64{"Put": 3, "Range": 1},
		ErrorCount:     0,
	}
}

func TestClientHistoryRepo_SaveAndList(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewClientHistoryRepo(store)
	ctx := context.Background()

	t0 := time.Now().Truncate(time.Second)

	require.NoError(t, repo.Save(ctx, makeSnap("a", t0)))
	require.NoError(t, repo.Save(ctx, makeSnap("b", t0.Add(1*time.Second))))
	require.NoError(t, repo.Save(ctx, makeSnap("c", t0.Add(2*time.Second))))

	got, err := repo.List(ctx, 0)
	require.NoError(t, err)
	require.Len(t, got, 3)

	// Newest first
	assert.Equal(t, "c", got[0].ID)
	assert.Equal(t, "b", got[1].ID)
	assert.Equal(t, "a", got[2].ID)

	assert.Equal(t, "svc", got[0].ClientName)
	assert.Equal(t, int64(3), got[0].RequestCounts["Put"])
}

func TestClientHistoryRepo_RequiresDisconnectedAt(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewClientHistoryRepo(store)

	err := repo.Save(context.Background(), &domain.Client{ID: "x"})
	require.Error(t, err)
}

func TestClientHistoryRepo_List_RespectsLimit(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewClientHistoryRepo(store)
	ctx := context.Background()

	t0 := time.Now()
	for i := range 5 {
		require.NoError(t, repo.Save(ctx, makeSnap("c", t0.Add(time.Duration(i)*time.Second))))
	}

	got, err := repo.List(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestClientHistoryRepo_Count(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewClientHistoryRepo(store)
	ctx := context.Background()

	n, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	require.NoError(t, repo.Save(ctx, makeSnap("a", time.Now())))
	require.NoError(t, repo.Save(ctx, makeSnap("b", time.Now().Add(time.Second))))

	n, err = repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
}

func TestClientHistoryRepo_DeleteOldest(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewClientHistoryRepo(store)
	ctx := context.Background()

	t0 := time.Now()
	for i := range 5 {
		require.NoError(t, repo.Save(ctx, makeSnap("c", t0.Add(time.Duration(i)*time.Second))))
	}

	deleted, err := repo.DeleteOldest(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, 3, deleted)

	got, _ := repo.List(ctx, 0)
	require.Len(t, got, 2, "2 newest remain")
}

func TestClientHistoryRepo_DeleteOldest_LimitedByAvailable(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewClientHistoryRepo(store)
	ctx := context.Background()

	require.NoError(t, repo.Save(ctx, makeSnap("a", time.Now())))

	// Asking for more than exists is fine — just deletes everything.
	deleted, err := repo.DeleteOldest(ctx, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)
}

func TestClientHistoryRepo_DeleteOldest_Zero_NoOp(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewClientHistoryRepo(store)
	ctx := context.Background()

	require.NoError(t, repo.Save(ctx, makeSnap("a", time.Now())))

	deleted, err := repo.DeleteOldest(ctx, 0)
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)
}

func TestClientHistoryRepo_DeleteOlderThan(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewClientHistoryRepo(store)
	ctx := context.Background()

	t0 := time.Now()
	require.NoError(t, repo.Save(ctx, makeSnap("old1", t0.Add(-2*time.Hour))))
	require.NoError(t, repo.Save(ctx, makeSnap("old2", t0.Add(-1*time.Hour))))
	require.NoError(t, repo.Save(ctx, makeSnap("recent", t0.Add(-5*time.Minute))))

	cutoff := t0.Add(-30 * time.Minute)
	deleted, err := repo.DeleteOlderThan(ctx, cutoff)
	require.NoError(t, err)
	assert.Equal(t, 2, deleted)

	got, _ := repo.List(ctx, 0)
	require.Len(t, got, 1)
	assert.Equal(t, "recent", got[0].ID)
}

func TestClientHistoryRepo_DeleteOlderThan_KeepsAll_WhenCutoffOldEnough(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewClientHistoryRepo(store)
	ctx := context.Background()

	t0 := time.Now()
	require.NoError(t, repo.Save(ctx, makeSnap("a", t0)))

	deleted, err := repo.DeleteOlderThan(ctx, t0.Add(-1*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)
}

func TestClientHistoryRepo_SameNanos_BothPersisted(t *testing.T) {
	t.Parallel()
	// Disambiguation by ID suffix on same-nano keys.
	store := newTestStore(t)
	repo := bboltadapter.NewClientHistoryRepo(store)
	ctx := context.Background()

	t0 := time.Now()
	require.NoError(t, repo.Save(ctx, makeSnap("a", t0)))
	require.NoError(t, repo.Save(ctx, makeSnap("b", t0)))

	got, _ := repo.List(ctx, 0)
	require.Len(t, got, 2, "both same-nano snapshots persisted via ID disambiguation")
}

func TestClientHistoryRepo_ListByClient(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewClientHistoryRepo(store)
	ctx := context.Background()

	t0 := time.Now()
	saved := []*domain.Client{
		mkClientSnap("a", "order-service", "production", t0),
		mkClientSnap("b", "order-service", "staging", t0.Add(time.Second)),
		mkClientSnap("c", "payment-service", "production", t0.Add(2*time.Second)),
		mkClientSnap("d", "order-service", "production", t0.Add(3*time.Second)),
	}
	for _, c := range saved {
		require.NoError(t, repo.Save(ctx, c))
	}

	t.Run("matches name+namespace, newest first", func(t *testing.T) {
		t.Parallel()
		got, err := repo.ListByClient(ctx, "order-service", "production", 0)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "d", got[0].ID)
		assert.Equal(t, "a", got[1].ID)
	})

	t.Run("respects limit", func(t *testing.T) {
		t.Parallel()
		got, err := repo.ListByClient(ctx, "order-service", "production", 1)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "d", got[0].ID)
	})

	t.Run("no matches → empty", func(t *testing.T) {
		t.Parallel()
		got, err := repo.ListByClient(ctx, "ghost", "nowhere", 0)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("empty namespace must match exactly", func(t *testing.T) {
		t.Parallel()
		// Sanity: namespace mismatch doesn't fall through to "any namespace".
		got, err := repo.ListByClient(ctx, "order-service", "", 0)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func mkClientSnap(id, name, ns string, disconnectedAt time.Time) *domain.Client {
	return &domain.Client{
		ID:             id,
		ClientName:     name,
		K8sNamespace:   ns,
		PeerAddress:    "10.0.0.1:1234",
		ConnectedAt:    disconnectedAt.Add(-time.Minute),
		DisconnectedAt: new(disconnectedAt),
		LastActivityAt: disconnectedAt,
	}
}

func TestClientHistoryRepo_PreservesAllFields(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewClientHistoryRepo(store)
	ctx := context.Background()

	d := time.Now().Truncate(time.Second).UTC()
	in := &domain.Client{
		ID:             "conn-1",
		PeerAddress:    "10.0.0.5:54321",
		UserAgent:      "ua",
		ClientName:     "order-service",
		ClientVersion:  "1.2.3",
		K8sNamespace:   "production",
		K8sPod:         "pod-1",
		K8sNode:        "node-1",
		InstanceID:     "instance-1",
		ConnectedAt:    d.Add(-time.Hour).UTC(),
		DisconnectedAt: &d,
		LastActivityAt: d.Add(-time.Minute).UTC(),
		ActiveWatches:  3,
		RequestCounts:  map[string]int64{"Put": 10},
		ErrorCount:     2,
	}

	require.NoError(t, repo.Save(ctx, in))

	got, err := repo.List(ctx, 1)
	require.NoError(t, err)
	require.Len(t, got, 1)

	out := got[0]
	assert.Equal(t, in.ID, out.ID)
	assert.Equal(t, in.PeerAddress, out.PeerAddress)
	assert.Equal(t, in.UserAgent, out.UserAgent)
	assert.Equal(t, in.ClientName, out.ClientName)
	assert.Equal(t, in.ClientVersion, out.ClientVersion)
	assert.Equal(t, in.K8sNamespace, out.K8sNamespace)
	assert.Equal(t, in.K8sPod, out.K8sPod)
	assert.Equal(t, in.K8sNode, out.K8sNode)
	assert.Equal(t, in.InstanceID, out.InstanceID)
	assert.True(t, in.ConnectedAt.Equal(out.ConnectedAt))
	assert.True(t, in.DisconnectedAt.Equal(*out.DisconnectedAt))
	assert.True(t, in.LastActivityAt.Equal(out.LastActivityAt))
	assert.Equal(t, in.ActiveWatches, out.ActiveWatches)
	assert.Equal(t, in.RequestCounts, out.RequestCounts)
	assert.Equal(t, in.ErrorCount, out.ErrorCount)
}
