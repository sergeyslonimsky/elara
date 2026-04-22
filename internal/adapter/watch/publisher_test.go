package watch_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	watchadapter "github.com/sergeyslonimsky/elara/internal/adapter/watch"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

func recvEvent(t *testing.T, ch <-chan domain.WatchEvent) (domain.WatchEvent, bool) {
	t.Helper()

	select {
	case ev, ok := <-ch:
		return ev, ok
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")

		return domain.WatchEvent{}, false
	}
}

func assertNoEventWithin(t *testing.T, ch <-chan domain.WatchEvent) {
	t.Helper()

	select {
	case ev := <-ch:
		t.Fatalf("unexpected event: %+v", ev)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestPublisher_NotifyCreated_DeliversEvent(t *testing.T) {
	t.Parallel()

	p := watchadapter.NewPublisher()
	defer p.Shutdown()

	ctx := context.Background()
	events, cleanup := p.Subscribe(ctx, "", "default")
	defer cleanup()

	cfg := &domain.Config{
		Path:      "/foo",
		Namespace: "default",
		Content:   "v1",
		Revision:  5,
	}
	p.NotifyCreated(ctx, cfg)

	ev, ok := recvEvent(t, events)
	require.True(t, ok)
	assert.Equal(t, domain.EventTypeCreated, ev.Type)
	assert.Equal(t, "/foo", ev.Path)
	assert.Equal(t, "default", ev.Namespace)
	assert.Equal(t, int64(5), ev.Revision, "Revision copied from Config")
	require.NotNil(t, ev.Config)
}

func TestPublisher_NotifyUpdated_DeliversEvent(t *testing.T) {
	t.Parallel()

	p := watchadapter.NewPublisher()
	defer p.Shutdown()

	ctx := context.Background()
	events, cleanup := p.Subscribe(ctx, "", "default")
	defer cleanup()

	cfg := &domain.Config{Path: "/foo", Namespace: "default", Revision: 2}
	p.NotifyUpdated(ctx, cfg)

	ev, ok := recvEvent(t, events)
	require.True(t, ok)
	assert.Equal(t, domain.EventTypeUpdated, ev.Type)
	assert.Equal(t, int64(2), ev.Revision)
}

func TestPublisher_NotifyDeleted_CarriesRevision(t *testing.T) {
	t.Parallel()

	p := watchadapter.NewPublisher()
	defer p.Shutdown()

	ctx := context.Background()
	events, cleanup := p.Subscribe(ctx, "", "default")
	defer cleanup()

	p.NotifyDeleted(ctx, "/foo", "default", 7)

	ev, ok := recvEvent(t, events)
	require.True(t, ok)
	assert.Equal(t, domain.EventTypeDeleted, ev.Type)
	assert.Equal(t, "/foo", ev.Path)
	assert.Equal(t, int64(7), ev.Revision, "delete revision must be propagated")
	assert.Nil(t, ev.Config)
}

func TestPublisher_NamespaceFilter(t *testing.T) {
	t.Parallel()

	p := watchadapter.NewPublisher()
	defer p.Shutdown()

	ctx := context.Background()
	events, cleanup := p.Subscribe(ctx, "", "default")
	defer cleanup()

	p.NotifyCreated(ctx, &domain.Config{Path: "/a", Namespace: "prod", Revision: 1})
	assertNoEventWithin(t, events)

	p.NotifyCreated(ctx, &domain.Config{Path: "/a", Namespace: "default", Revision: 2})
	ev, ok := recvEvent(t, events)
	require.True(t, ok)
	assert.Equal(t, "default", ev.Namespace)
}

func TestPublisher_PathPrefixFilter(t *testing.T) {
	t.Parallel()

	p := watchadapter.NewPublisher()
	defer p.Shutdown()

	ctx := context.Background()
	events, cleanup := p.Subscribe(ctx, "/services/", "default")
	defer cleanup()

	p.NotifyCreated(ctx, &domain.Config{Path: "/other", Namespace: "default", Revision: 1})
	assertNoEventWithin(t, events)

	p.NotifyCreated(ctx, &domain.Config{Path: "/services/api", Namespace: "default", Revision: 2})
	ev, ok := recvEvent(t, events)
	require.True(t, ok)
	assert.Equal(t, "/services/api", ev.Path)
}

func TestPublisher_EmptyFilters_MatchEverything(t *testing.T) {
	t.Parallel()

	p := watchadapter.NewPublisher()
	defer p.Shutdown()

	ctx := context.Background()
	events, cleanup := p.Subscribe(ctx, "", "")
	defer cleanup()

	p.NotifyCreated(ctx, &domain.Config{Path: "/a", Namespace: "default", Revision: 1})
	p.NotifyCreated(ctx, &domain.Config{Path: "/b", Namespace: "prod", Revision: 2})

	ev1, ok := recvEvent(t, events)
	require.True(t, ok)
	ev2, ok := recvEvent(t, events)
	require.True(t, ok)

	namespaces := map[string]bool{ev1.Namespace: true, ev2.Namespace: true}
	assert.True(t, namespaces["default"])
	assert.True(t, namespaces["prod"])
}

func TestPublisher_Cleanup_IsIdempotent(t *testing.T) {
	t.Parallel()

	p := watchadapter.NewPublisher()
	defer p.Shutdown()

	ctx := context.Background()
	events, cleanup := p.Subscribe(ctx, "", "ns")

	cleanup()
	cleanup() // must not panic

	// Channel is closed — reading should drain and report closed.
	_, ok := <-events
	assert.False(t, ok, "channel closed after cleanup")
}

func TestPublisher_Cleanup_UnsubscribesFromFurtherNotifications(t *testing.T) {
	t.Parallel()

	p := watchadapter.NewPublisher()
	defer p.Shutdown()

	ctx := context.Background()
	events, cleanup := p.Subscribe(ctx, "", "ns")

	cleanup()

	// Publishing after cleanup must not send on a closed channel (would panic).
	p.NotifyCreated(ctx, &domain.Config{Path: "/a", Namespace: "ns", Revision: 1})

	// Draining must show channel closed.
	_, ok := <-events
	assert.False(t, ok)
}

func TestPublisher_MultipleSubscribers_IndependentDelivery(t *testing.T) {
	t.Parallel()

	p := watchadapter.NewPublisher()
	defer p.Shutdown()

	ctx := context.Background()

	ev1, c1 := p.Subscribe(ctx, "", "ns1")
	defer c1()
	ev2, c2 := p.Subscribe(ctx, "", "ns2")
	defer c2()

	p.NotifyCreated(ctx, &domain.Config{Path: "/x", Namespace: "ns1", Revision: 1})
	p.NotifyCreated(ctx, &domain.Config{Path: "/y", Namespace: "ns2", Revision: 2})

	e1, ok := recvEvent(t, ev1)
	require.True(t, ok)
	assert.Equal(t, "ns1", e1.Namespace)

	e2, ok := recvEvent(t, ev2)
	require.True(t, ok)
	assert.Equal(t, "ns2", e2.Namespace)

	// Cross-subscriber leakage check
	assertNoEventWithin(t, ev1)
	assertNoEventWithin(t, ev2)
}

func TestPublisher_Shutdown_ClosesAllSubscribers(t *testing.T) {
	t.Parallel()

	p := watchadapter.NewPublisher()

	ctx := context.Background()
	ev, _ := p.Subscribe(ctx, "", "ns")

	p.Shutdown()

	_, ok := <-ev
	assert.False(t, ok, "Shutdown must close subscriber channels")
}

func TestPublisher_BufferFull_DropsWithoutBlocking(t *testing.T) {
	t.Parallel()

	// This test confirms that a slow consumer doesn't block the publisher.
	// We subscribe but don't read from the channel, then send more than the
	// buffer capacity of notifications (default 100).
	p := watchadapter.NewPublisher()
	defer p.Shutdown()

	ctx := context.Background()
	_, cleanup := p.Subscribe(ctx, "", "ns")
	defer cleanup()

	// Fire many events — publisher must not block even with a full buffer.
	done := make(chan struct{})
	go func() {
		for i := range 500 {
			p.NotifyCreated(ctx, &domain.Config{Path: "/x", Namespace: "ns", Revision: int64(i)})
		}

		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publisher blocked when buffer was full — events should be dropped instead")
	}
}

func TestPublisher_NotifyConfigLocked_DeliversLockedEvent(t *testing.T) {
	t.Parallel()

	p := watchadapter.NewPublisher()
	defer p.Shutdown()

	ctx := context.Background()
	events, cleanup := p.Subscribe(ctx, "", "prod")
	defer cleanup()

	cfg := &domain.Config{Path: "/a.json", Namespace: "prod", Locked: true, Revision: 7}
	p.NotifyConfigLocked(ctx, cfg)

	ev, ok := recvEvent(t, events)
	require.True(t, ok)
	assert.Equal(t, domain.EventTypeLocked, ev.Type)
	assert.Equal(t, "/a.json", ev.Path)
	require.NotNil(t, ev.Config)
	assert.True(t, ev.Config.Locked)
}

func TestPublisher_NotifyNamespaceLocked_DeliveredToPathFilteredSubscribers(t *testing.T) {
	t.Parallel()

	p := watchadapter.NewPublisher()
	defer p.Shutdown()

	ctx := context.Background()

	// Subscriber with a narrow path prefix — would normally not match namespace
	// events (path == ""), but the publisher special-cases namespace-level events.
	events, cleanup := p.Subscribe(ctx, "/services/", "prod")
	defer cleanup()

	p.NotifyNamespaceLocked(ctx, "prod")

	ev, ok := recvEvent(t, events)
	require.True(t, ok)
	assert.Equal(t, domain.EventTypeNamespaceLocked, ev.Type)
	assert.Equal(t, "prod", ev.Namespace)
	assert.Empty(t, ev.Path)
}

func TestPublisher_NotifyNamespaceLocked_FiltersOtherNamespaces(t *testing.T) {
	t.Parallel()

	p := watchadapter.NewPublisher()
	defer p.Shutdown()

	ctx := context.Background()
	events, cleanup := p.Subscribe(ctx, "", "staging")
	defer cleanup()

	p.NotifyNamespaceLocked(ctx, "prod")

	assertNoEventWithin(t, events)
}
