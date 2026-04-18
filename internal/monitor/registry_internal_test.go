package monitor

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// recordingSink captures every disconnect snapshot for assertions.
type recordingSink struct {
	mu       sync.Mutex
	captured []*domain.Client
}

func (s *recordingSink) Record(c *domain.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.captured = append(s.captured, c)
}

func (s *recordingSink) snapshot() []*domain.Client {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]*domain.Client, len(s.captured))
	copy(out, s.captured)

	return out
}

// drainEvents reads up to n change events from ch, with timeout.
func drainEvents(t *testing.T, ch <-chan domain.ClientChange, n int) []domain.ClientChange {
	t.Helper()

	out := make([]domain.ClientChange, 0, n)
	deadline := time.After(time.Second)

	for len(out) < n {
		select {
		case ev, ok := <-ch:
			if !ok {
				return out
			}

			out = append(out, ev)
		case <-deadline:
			t.Fatalf("timed out waiting for %d events (got %d)", n, len(out))
		}
	}

	return out
}

// -----------------------------------------------------------------------------
// Registration & identity
// -----------------------------------------------------------------------------

func TestRegistry_Register_AssignsMonotonicID(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)

	id1 := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "1.1.1.1:1"})
	id2 := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "2.2.2.2:2"})

	assert.Equal(t, "conn-1", id1)
	assert.Equal(t, "conn-2", id2)
}

func TestRegistry_Register_PopulatesSnapshot(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)

	id := r.RegisterConnection(domain.ConnectionInfo{
		PeerAddress: "10.0.0.5:54321",
		UserAgent:   "etcd-client/3.5.0",
		ClientName:  "order-service",
	})

	c := r.Get(id)
	require.NotNil(t, c)
	assert.Equal(t, id, c.ID)
	assert.Equal(t, "10.0.0.5:54321", c.PeerAddress)
	assert.Equal(t, "etcd-client/3.5.0", c.UserAgent)
	assert.Equal(t, "order-service", c.ClientName)
	assert.True(t, c.IsActive())
	assert.Nil(t, c.DisconnectedAt)
	assert.NotZero(t, c.ConnectedAt)
}

func TestRegistry_Get_Missing(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)
	assert.Nil(t, r.Get("does-not-exist"))
}

func TestRegistry_UpdateIdentity_OnlyFirstWins(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)
	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})

	r.UpdateIdentity(id, domain.ConnectionInfo{ClientName: "first"})
	r.UpdateIdentity(id, domain.ConnectionInfo{ClientName: "second"})

	c := r.Get(id)
	assert.Equal(t, "first", c.ClientName, "identity must be set once and remain stable")
}

func TestRegistry_UpdateIdentity_UnknownConn_NoOp(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)
	// must not panic
	r.UpdateIdentity("nope", domain.ConnectionInfo{ClientName: "x"})
}

func TestRegistry_RegisterPopulatesIdentityImmediately(t *testing.T) {
	t.Parallel()

	// If the connection arrives with metadata already (rare but possible),
	// the first snapshot should reflect it without requiring UpdateIdentity.
	r := NewRegistry(Config{}, nil)

	id := r.RegisterConnection(domain.ConnectionInfo{
		PeerAddress: "p",
		ClientName:  "x",
	})

	r.UpdateIdentity(id, domain.ConnectionInfo{ClientName: "should-not-overwrite"})

	c := r.Get(id)
	assert.Equal(t, "x", c.ClientName)
}

// -----------------------------------------------------------------------------
// Counters & events
// -----------------------------------------------------------------------------

func TestRegistry_RecordRequest_IncCountersAndEvents(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{DisableActivityEvents: true}, nil) // disable activity throttle to silence pubs
	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})

	r.RecordRequest(id, "Put", "/ns/a", 1, 5*time.Millisecond, nil)
	r.RecordRequest(id, "Put", "/ns/b", 2, 7*time.Millisecond, nil)
	r.RecordRequest(id, "Range", "/ns/", 0, 1*time.Millisecond, nil)
	r.RecordRequest(id, "Put", "/ns/c", 3, 4*time.Millisecond, errors.New("boom"))

	c := r.Get(id)
	assert.Equal(t, int64(3), c.RequestCounts["Put"])
	assert.Equal(t, int64(1), c.RequestCounts["Range"])
	assert.Equal(t, int64(1), c.ErrorCount)

	events := r.RecentEvents(id)
	require.Len(t, events, 4)
	assert.Equal(t, "Put", events[0].Method)
	assert.Equal(t, "boom", events[3].Error)
}

func TestRegistry_RecordRequest_UnknownConn_NoOp(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)
	r.RecordRequest("nope", "Put", "", 0, 0, nil)
}

func TestRegistry_RecentEvents_RingBufferLimit(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{RecentEventsCapacity: 3}, nil)
	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})

	for i := range 5 {
		r.RecordRequest(id, "Put", "", int64(i), 0, nil)
	}

	events := r.RecentEvents(id)
	require.Len(t, events, 3)
	assert.Equal(t, int64(2), events[0].Revision, "oldest two evicted")
	assert.Equal(t, int64(4), events[2].Revision, "newest preserved")
}

func TestRegistry_RegisterWatch_TracksDetailAndCount(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)
	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})

	now := time.Now()
	r.RegisterWatch(id, domain.ActiveWatch{
		WatchID:       1,
		StartKey:      "/default/foo",
		StartRevision: 5,
		CreatedAt:     now,
		PrevKv:        true,
	})
	r.RegisterWatch(id, domain.ActiveWatch{
		WatchID:       2,
		StartKey:      "/default/",
		EndKey:        "/default0",
		StartRevision: 1,
		CreatedAt:     now,
	})

	c := r.Get(id)
	require.NotNil(t, c)
	assert.Equal(t, int32(2), c.ActiveWatches, "count derived from watches map")
	require.Len(t, c.ActiveWatchList, 2)

	got := r.ActiveWatches(id)
	require.Len(t, got, 2)

	byID := map[int64]domain.ActiveWatch{}
	for _, w := range got {
		byID[w.WatchID] = w
	}
	assert.Equal(t, "/default/foo", byID[1].StartKey)
	assert.Equal(t, "/default/", byID[2].StartKey)
	assert.Equal(t, "/default0", byID[2].EndKey)
}

func TestRegistry_UnregisterWatch(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)
	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})

	r.RegisterWatch(id, domain.ActiveWatch{WatchID: 1, StartKey: "/a"})
	r.RegisterWatch(id, domain.ActiveWatch{WatchID: 2, StartKey: "/b"})

	r.UnregisterWatch(id, 1)
	assert.Equal(t, int32(1), r.Get(id).ActiveWatches)

	got := r.ActiveWatches(id)
	require.Len(t, got, 1)
	assert.Equal(t, int64(2), got[0].WatchID)

	// Unknown ID — no-op
	r.UnregisterWatch(id, 999)
	assert.Equal(t, int32(1), r.Get(id).ActiveWatches)
}

func TestRegistry_RegisterWatch_Idempotent(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)
	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})

	r.RegisterWatch(id, domain.ActiveWatch{WatchID: 1, StartKey: "/a"})
	r.RegisterWatch(id, domain.ActiveWatch{WatchID: 1, StartKey: "/a-updated"})

	got := r.ActiveWatches(id)
	require.Len(t, got, 1, "same WatchID overwrites, doesn't duplicate")
	assert.Equal(t, "/a-updated", got[0].StartKey)
	assert.Equal(t, int32(1), r.Get(id).ActiveWatches)
}

func TestRegistry_ActiveWatches_UnknownConn(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)
	assert.Nil(t, r.ActiveWatches("nope"))
}

func TestRegistry_ActiveWatches(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)
	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})

	r.IncActiveWatches(id)
	r.IncActiveWatches(id)
	assert.Equal(t, int32(2), r.Get(id).ActiveWatches)

	r.DecActiveWatches(id)
	assert.Equal(t, int32(1), r.Get(id).ActiveWatches)

	// Unknown id is a no-op
	r.IncActiveWatches("nope")
}

// -----------------------------------------------------------------------------
// History sink
// -----------------------------------------------------------------------------

func TestRegistry_Unregister_RecordsToHistorySinkWithDisconnectedAt(t *testing.T) {
	t.Parallel()

	sink := &recordingSink{}
	r := NewRegistry(Config{}, sink)

	id := r.RegisterConnection(domain.ConnectionInfo{
		PeerAddress: "p", ClientName: "svc",
	})
	r.RecordRequest(id, "Put", "", 0, 0, nil)

	r.UnregisterConnection(id)

	captured := sink.snapshot()
	require.Len(t, captured, 1)

	c := captured[0]
	assert.Equal(t, "svc", c.ClientName)
	assert.Equal(t, int64(1), c.RequestCounts["Put"])
	require.NotNil(t, c.DisconnectedAt, "DisconnectedAt set in history snapshot")
	assert.False(t, c.IsActive())

	// Subsequent Get returns nil — no longer active
	assert.Nil(t, r.Get(id))

	// Idempotent — second unregister is a no-op
	r.UnregisterConnection(id)
	assert.Len(t, sink.snapshot(), 1, "no duplicate history record on double-unregister")
}

func TestRegistry_NilHistorySink_Safe(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)

	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})
	r.UnregisterConnection(id) // must not panic
}

// -----------------------------------------------------------------------------
// Pub/Sub
// -----------------------------------------------------------------------------

func TestRegistry_PubSub_ConnectAndDisconnect(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)
	defer r.Shutdown()

	ch, cleanup := r.Subscribe()
	defer cleanup()

	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p", ClientName: "svc"})
	r.UnregisterConnection(id)

	events := drainEvents(t, ch, 2)
	assert.Equal(t, domain.ClientConnected, events[0].Kind)
	assert.Equal(t, "svc", events[0].Client.ClientName)
	assert.Equal(t, domain.ClientDisconnected, events[1].Kind)
	require.NotNil(t, events[1].Client.DisconnectedAt)
}

func TestRegistry_PubSub_ActivityIsThrottled(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{ActivityThrottle: 100 * time.Millisecond}, nil)
	defer r.Shutdown()

	ch, cleanup := r.Subscribe()
	defer cleanup()

	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})

	// Drain the Connected event.
	drainEvents(t, ch, 1)

	// Burst of 100 RPCs in rapid succession should produce ≤ a few activity events.
	for range 100 {
		r.RecordRequest(id, "Put", "", 0, 0, nil)
	}

	// Drain whatever activity events arrived. Could be 0 or 1 depending on timing.
	got := 0
loop:
	for {
		select {
		case ev := <-ch:
			if ev.Kind == domain.ClientActivity {
				got++
			}
		case <-time.After(50 * time.Millisecond):
			break loop
		}
	}

	assert.LessOrEqual(t, got, 2, "throttle must collapse 100 RPCs into ≤ 2 activity events")
}

func TestRegistry_PubSub_ActivityDisabledWhenFlagSet(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{DisableActivityEvents: true}, nil)
	defer r.Shutdown()

	ch, cleanup := r.Subscribe()
	defer cleanup()

	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})
	drainEvents(t, ch, 1) // Connected

	for range 50 {
		r.RecordRequest(id, "Put", "", 0, 0, nil)
	}

	// Should see no Activity events.
	deadline := time.After(100 * time.Millisecond)
	for {
		select {
		case ev := <-ch:
			assert.NotEqual(t, domain.ClientActivity, ev.Kind, "Activity events disabled but received one")
		case <-deadline:
			return
		}
	}
}

func TestRegistry_Subscribe_Cleanup_IsIdempotent(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)
	defer r.Shutdown()

	ch, cleanup := r.Subscribe()

	cleanup()
	cleanup() // must not panic

	_, ok := <-ch
	assert.False(t, ok, "channel closed after cleanup")
}

func TestRegistry_Shutdown_ClosesAllSubscriptions(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)

	ch, _ := r.Subscribe()
	r.Shutdown()

	_, ok := <-ch
	assert.False(t, ok, "Shutdown closes subscriber channels")

	// Subscribe-after-shutdown returns a closed channel so subscribers don't hang.
	ch2, _ := r.Subscribe()

	_, ok = <-ch2
	assert.False(t, ok)
}

func TestRegistry_PubSub_DropsOnFullBuffer(t *testing.T) {
	t.Parallel()

	// Subscribe but never drain — publisher must continue to function for
	// other subscribers and must not block on RegisterConnection.
	r := NewRegistry(Config{}, nil)
	defer r.Shutdown()

	_, cleanup := r.Subscribe()
	defer cleanup()

	done := make(chan struct{})
	go func() {
		// Far more than the publisher's buffer.
		for range 2*defaultPublisherBuffer + 10 {
			id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})
			r.UnregisterConnection(id)
		}

		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publisher blocked when subscriber buffer was full")
	}
}

// -----------------------------------------------------------------------------
// Concurrency
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// SubscribeClient (per-client publisher)
// -----------------------------------------------------------------------------

func TestRegistry_SubscribeClient_DeliversPerRPCEvents(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{DisableActivityEvents: true}, nil)
	defer r.Shutdown()

	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})

	ch, cleanup := r.SubscribeClient(id)
	defer cleanup()

	r.RecordRequest(id, "Put", "/foo", 5, 1*time.Millisecond, nil)
	r.RecordRequest(id, "Range", "/", 0, 2*time.Millisecond, nil)

	events := drainEvents(t, ch, 2)
	for _, e := range events {
		assert.Equal(t, domain.ClientRequestRecorded, e.Kind)
		assert.NotNil(t, e.Event, "Event payload populated for per-RPC events")
	}
	assert.Equal(t, "Put", events[0].Event.Method)
	assert.Equal(t, "Range", events[1].Event.Method)
}

func TestRegistry_SubscribeClient_DisconnectClosesChannel(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)
	defer r.Shutdown()

	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})
	ch, cleanup := r.SubscribeClient(id)
	defer cleanup()

	r.UnregisterConnection(id)

	// Should receive Disconnected then channel closes.
	events := drainEvents(t, ch, 1)
	require.Len(t, events, 1)
	assert.Equal(t, domain.ClientDisconnected, events[0].Kind)

	// Next read should observe the closed channel.
	deadline := time.After(time.Second)
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel closed after disconnect")
	case <-deadline:
		t.Fatal("channel was not closed after disconnect")
	}
}

func TestRegistry_SubscribeClient_UnknownConn_ReturnsClosedChan(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)
	defer r.Shutdown()

	ch, cleanup := r.SubscribeClient("does-not-exist")
	defer cleanup()

	_, ok := <-ch
	assert.False(t, ok, "missing client → closed channel, no hang")
}

func TestRegistry_SubscribeClient_NoSubscribers_NoPublishOverhead(t *testing.T) {
	t.Parallel()

	// Hot-path guard: when nobody subscribes, RecordRequest must not allocate
	// per-RPC publisher. We assert this indirectly: e.pub stays nil.
	r := NewRegistry(Config{DisableActivityEvents: true}, nil)
	defer r.Shutdown()

	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})

	for range 100 {
		r.RecordRequest(id, "Put", "", 0, 0, nil)
	}

	// peek at internal state via lookup() — pub must remain nil
	e := r.lookup(id)
	require.NotNil(t, e)

	e.pubMu.RLock()
	defer e.pubMu.RUnlock()
	assert.Nil(t, e.pub, "no SubscribeClient → no publisher allocation")
}

func TestRegistry_SubscribeClient_Cleanup_ReleasesSubscription(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)
	defer r.Shutdown()

	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})
	ch, cleanup := r.SubscribeClient(id)

	cleanup()

	_, ok := <-ch
	assert.False(t, ok, "cleanup must close channel")

	// Repeated cleanup must not panic.
	cleanup()
}

func TestRegistry_SubscribeClient_Shutdown_ClosesPerClientChans(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{}, nil)

	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})
	ch, _ := r.SubscribeClient(id)

	r.Shutdown()

	deadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("Shutdown did not close per-client subscriber channel")
		}
	}
}

func TestRegistry_SubscribeClient_BurstDoesNotBlockHotPath(t *testing.T) {
	t.Parallel()

	// A subscriber that doesn't drain → publisher must drop, not block RecordRequest.
	r := NewRegistry(Config{DisableActivityEvents: true}, nil)
	defer r.Shutdown()

	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})
	_, cleanup := r.SubscribeClient(id)
	defer cleanup()

	done := make(chan struct{})
	go func() {
		for range 10 * defaultPublisherBuffer {
			r.RecordRequest(id, "Put", "", 0, 0, nil)
		}

		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RecordRequest blocked when subscriber buffer was full")
	}
}

func TestRegistry_ConcurrentRecordRequest(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{DisableActivityEvents: true}, nil)
	id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})

	var wg sync.WaitGroup
	const goroutines = 20
	const perGoroutine = 100

	for range goroutines {
		wg.Go(func() {
			for range perGoroutine {
				r.RecordRequest(id, "Put", "", 0, 0, nil)
			}
		})
	}

	wg.Wait()

	c := r.Get(id)
	assert.Equal(t, int64(goroutines*perGoroutine), c.RequestCounts["Put"])
}

func TestRegistry_ConcurrentRegisterAndQuery(t *testing.T) {
	t.Parallel()

	r := NewRegistry(Config{DisableActivityEvents: true}, nil)
	defer r.Shutdown()

	var (
		stop atomic.Bool
		wg   sync.WaitGroup
	)

	wg.Go(func() {
		for !stop.Load() {
			id := r.RegisterConnection(domain.ConnectionInfo{PeerAddress: "p"})
			r.RecordRequest(id, "Put", "", 0, 0, nil)
			r.UnregisterConnection(id)
		}
	})

	wg.Go(func() {
		for range 1000 {
			_ = r.ListActive()
		}
	})

	time.Sleep(50 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
}
