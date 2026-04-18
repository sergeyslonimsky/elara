package monitor_test

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/monitor"
)

// fakeHistoryRepo is an in-memory ClientHistoryRepo for unit testing the
// async HistoryStore wrapper. Operations are guarded by a single mutex.
type fakeHistoryRepo struct {
	mu       sync.Mutex
	saved    []*domain.Client // newest-first invariant maintained on insert
	saveErr  error
	countErr error

	saveCalls         int
	deleteOldestCalls int
	deleteOlderCalls  int
}

func (r *fakeHistoryRepo) Save(_ context.Context, c *domain.Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.saveCalls++

	if r.saveErr != nil {
		return r.saveErr
	}

	r.saved = append(r.saved, c)
	// keep saved sorted newest-first by DisconnectedAt
	sort.SliceStable(r.saved, func(i, j int) bool {
		return r.saved[i].DisconnectedAt.After(*r.saved[j].DisconnectedAt)
	})

	return nil
}

func (r *fakeHistoryRepo) ListByClient(_ context.Context, name, ns string, limit int) ([]*domain.Client, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var out []*domain.Client
	for _, c := range r.saved {
		if c.ClientName == name && c.K8sNamespace == ns {
			out = append(out, c)
		}
	}
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}

	return out, nil
}

func (r *fakeHistoryRepo) List(_ context.Context, limit int) ([]*domain.Client, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if limit <= 0 || limit >= len(r.saved) {
		out := make([]*domain.Client, len(r.saved))
		copy(out, r.saved)

		return out, nil
	}

	out := make([]*domain.Client, limit)
	copy(out, r.saved[:limit])

	return out, nil
}

func (r *fakeHistoryRepo) Count(_ context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.countErr != nil {
		return 0, r.countErr
	}

	return len(r.saved), nil
}

func (r *fakeHistoryRepo) DeleteOldest(_ context.Context, n int) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.deleteOldestCalls++

	if n <= 0 || len(r.saved) == 0 {
		return 0, nil
	}

	if n > len(r.saved) {
		n = len(r.saved)
	}

	r.saved = r.saved[:len(r.saved)-n] // tail of slice is the oldest

	return n, nil
}

func (r *fakeHistoryRepo) DeleteOlderThan(_ context.Context, cutoff time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.deleteOlderCalls++

	keep := r.saved[:0]
	for _, c := range r.saved {
		if c.DisconnectedAt.Before(cutoff) {
			continue
		}

		keep = append(keep, c)
	}

	deleted := len(r.saved) - len(keep)
	r.saved = keep

	return deleted, nil
}

func (r *fakeHistoryRepo) snapshot() []*domain.Client {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]*domain.Client, len(r.saved))
	copy(out, r.saved)

	return out
}

func mkSnap(id string, disconnectedAt time.Time) *domain.Client {
	return &domain.Client{ID: id, DisconnectedAt: new(disconnectedAt)}
}

// waitFor polls cond until true or fails after 1 second.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(2 * time.Millisecond)
	}

	t.Fatalf("condition never became true within %s", time.Second)
}

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

func TestHistoryStore_RecordPersistsAsync(t *testing.T) {
	t.Parallel()

	repo := &fakeHistoryRepo{}
	s := monitor.NewHistoryStore(t.Context(), monitor.HistoryConfig{}, repo)
	defer s.Shutdown()

	s.Record(mkSnap("a", time.Now()))
	s.Record(mkSnap("b", time.Now().Add(time.Second)))

	waitFor(t, func() bool { return len(repo.snapshot()) == 2 })
}

func TestHistoryStore_Record_DoesNotBlockOnFullBuffer(t *testing.T) {
	t.Parallel()

	// repo.Save blocks → buffer fills → further Record() calls drop, but never block.
	block := make(chan struct{})
	repo := &slowRepo{block: block}
	s := monitor.NewHistoryStore(t.Context(), monitor.HistoryConfig{BufferSize: 4}, repo)

	// Cleanup order matters: unblock the writer before Shutdown waits for it.
	t.Cleanup(func() {
		close(block)
		s.Shutdown()
	})

	done := make(chan struct{})
	go func() {
		for range 100 {
			s.Record(mkSnap("x", time.Now()))
		}

		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Record() blocked when async buffer was full")
	}
}

// slowRepo blocks Save until block channel is closed, simulating a slow disk.
type slowRepo struct {
	block chan struct{}
	fakeHistoryRepo
}

func (r *slowRepo) Save(ctx context.Context, c *domain.Client) error {
	<-r.block

	return r.fakeHistoryRepo.Save(ctx, c)
}

func TestHistoryStore_Shutdown_DrainsPending(t *testing.T) {
	t.Parallel()

	repo := &fakeHistoryRepo{}
	s := monitor.NewHistoryStore(t.Context(), monitor.HistoryConfig{BufferSize: 16}, repo)

	for i := range 10 {
		s.Record(mkSnap("x", time.Now().Add(time.Duration(i)*time.Millisecond)))
	}

	s.Shutdown()

	assert.Equal(t, 10, repo.saveCalls, "Shutdown must drain all pending writes before returning")
}

func TestHistoryStore_Shutdown_Idempotent(t *testing.T) {
	t.Parallel()

	s := monitor.NewHistoryStore(t.Context(), monitor.HistoryConfig{}, &fakeHistoryRepo{})

	s.Shutdown()
	s.Shutdown() // must not panic
}

func TestHistoryStore_Record_AfterShutdown_DoesNotBlock(t *testing.T) {
	t.Parallel()

	s := monitor.NewHistoryStore(t.Context(), monitor.HistoryConfig{}, &fakeHistoryRepo{})
	s.Shutdown()

	done := make(chan struct{})
	go func() {
		s.Record(mkSnap("x", time.Now()))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Record() blocked after Shutdown")
	}
}

// -----------------------------------------------------------------------------
// Retention
// -----------------------------------------------------------------------------

func TestHistoryStore_Retention_CountBased(t *testing.T) {
	t.Parallel()

	repo := &fakeHistoryRepo{}
	s := monitor.NewHistoryStore(t.Context(), monitor.HistoryConfig{
		MaxRecords: 3,
		MaxAge:     -1, // disable age retention
	}, repo)
	defer s.Shutdown()

	for i := range 5 {
		s.Record(mkSnap("x", time.Now().Add(time.Duration(i)*time.Millisecond)))
	}

	waitFor(t, func() bool { return len(repo.snapshot()) == 3 })
}

func TestHistoryStore_Retention_AgeBased(t *testing.T) {
	t.Parallel()

	repo := &fakeHistoryRepo{}
	s := monitor.NewHistoryStore(t.Context(), monitor.HistoryConfig{
		MaxRecords: 1000,
		MaxAge:     50 * time.Millisecond,
	}, repo)
	defer s.Shutdown()

	old := time.Now().Add(-time.Hour)
	recent := time.Now()
	s.Record(mkSnap("old", old))
	s.Record(mkSnap("recent", recent))

	waitFor(t, func() bool {
		snap := repo.snapshot()

		return len(snap) == 1 && snap[0].ID == "recent"
	})
}

func TestHistoryStore_Retention_DisabledAge(t *testing.T) {
	t.Parallel()

	repo := &fakeHistoryRepo{}
	s := monitor.NewHistoryStore(t.Context(), monitor.HistoryConfig{
		MaxRecords: 10,
		MaxAge:     -1, // disabled
	}, repo)
	defer s.Shutdown()

	old := time.Now().Add(-100 * 24 * time.Hour)
	s.Record(mkSnap("ancient", old))

	waitFor(t, func() bool { return len(repo.snapshot()) == 1 })
	assert.Equal(t, 0, repo.deleteOlderCalls, "MaxAge<0 must skip age retention")
}

func TestHistoryStore_Retention_BothMissingDefaults(t *testing.T) {
	t.Parallel()

	// Zero config → defaults applied: MaxRecords=1000, MaxAge=30d, BufferSize=256
	repo := &fakeHistoryRepo{}
	s := monitor.NewHistoryStore(t.Context(), monitor.HistoryConfig{}, repo)
	defer s.Shutdown()

	s.Record(mkSnap("x", time.Now()))

	waitFor(t, func() bool { return len(repo.snapshot()) == 1 })
}

// -----------------------------------------------------------------------------
// List passthrough + error tolerance
// -----------------------------------------------------------------------------

func TestHistoryStore_List_PassesThroughToRepo(t *testing.T) {
	t.Parallel()

	repo := &fakeHistoryRepo{}
	repo.saved = []*domain.Client{mkSnap("a", time.Now()), mkSnap("b", time.Now())}

	s := monitor.NewHistoryStore(t.Context(), monitor.HistoryConfig{}, repo)
	defer s.Shutdown()

	got, err := s.List(context.Background(), 1)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestHistoryStore_TolerantOfRepoErrors(t *testing.T) {
	t.Parallel()

	// Save errors must be logged but not crash the writer goroutine.
	repo := &fakeHistoryRepo{saveErr: errors.New("boom")}
	s := monitor.NewHistoryStore(t.Context(), monitor.HistoryConfig{}, repo)
	defer s.Shutdown()

	for range 3 {
		s.Record(mkSnap("x", time.Now()))
	}

	// Give it time to process.
	time.Sleep(50 * time.Millisecond)

	// No persisted entries (Save errored), but writer is still alive.
	repo.mu.Lock()
	assert.Equal(t, 3, repo.saveCalls)
	repo.mu.Unlock()
}
