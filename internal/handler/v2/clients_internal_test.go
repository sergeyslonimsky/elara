package v2

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	clientsv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/clients/v1"
	clientsuc "github.com/sergeyslonimsky/elara/internal/usecase/clients"
)

// fakeActiveSource implements clientsuc.ActiveSource.
type fakeActiveSource struct {
	mu              sync.Mutex
	clients         []*domain.Client
	events          map[string][]domain.ClientEvent
	subscribersMu   sync.Mutex
	subscribers     []chan domain.ClientChange
	subscribeCalls  int
	subscribeCancel int
}

func (s *fakeActiveSource) ListActive() []*domain.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*domain.Client, len(s.clients))
	copy(out, s.clients)

	return out
}

func (s *fakeActiveSource) Get(id string) *domain.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.clients {
		if c.ID == id {
			return c
		}
	}

	return nil
}

func (s *fakeActiveSource) RecentEvents(id string) []domain.ClientEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.events[id]
}

func (s *fakeActiveSource) Subscribe() (<-chan domain.ClientChange, func()) {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()

	s.subscribeCalls++
	ch := make(chan domain.ClientChange, 8)
	s.subscribers = append(s.subscribers, ch)

	cleanup := func() {
		s.subscribersMu.Lock()
		defer s.subscribersMu.Unlock()

		for i, c := range s.subscribers {
			if c == ch {
				s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
				close(ch)
				s.subscribeCancel++

				return
			}
		}
	}

	return ch, cleanup
}

// SubscribeClient — same channel pool as Subscribe so existing tests aren't affected.
func (s *fakeActiveSource) SubscribeClient(_ string) (<-chan domain.ClientChange, func()) {
	return s.Subscribe()
}

func (s *fakeActiveSource) push(ev domain.ClientChange) {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()
	for _, c := range s.subscribers {
		select {
		case c <- ev:
		default:
		}
	}
}

func (s *fakeActiveSource) activeSubs() int {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()

	return len(s.subscribers)
}

// fakeHistorySource implements clientsuc.HistorySource.
type fakeHistorySource struct {
	mu      sync.Mutex
	saved   []*domain.Client
	listErr error
}

func (h *fakeHistorySource) List(_ context.Context, limit int) ([]*domain.Client, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.listErr != nil {
		return nil, h.listErr
	}

	if limit <= 0 || limit > len(h.saved) {
		limit = len(h.saved)
	}

	out := make([]*domain.Client, limit)
	copy(out, h.saved[:limit])

	return out, nil
}

func (h *fakeHistorySource) ListByClient(_ context.Context, name, ns string, limit int) ([]*domain.Client, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.listErr != nil {
		return nil, h.listErr
	}

	var matches []*domain.Client
	for _, c := range h.saved {
		if c.ClientName == name && c.K8sNamespace == ns {
			matches = append(matches, c)
		}
	}

	if limit > 0 && limit < len(matches) {
		matches = matches[:limit]
	}

	return matches, nil
}

// fakeWatchSender captures sent responses for assertions.
type fakeWatchSender struct {
	mu      sync.Mutex
	resps   []*clientsv1.WatchClientsResponse
	sendErr error
}

func (s *fakeWatchSender) Send(r *clientsv1.WatchClientsResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sendErr != nil {
		return s.sendErr
	}

	s.resps = append(s.resps, r)

	return nil
}

func (s *fakeWatchSender) snapshot() []*clientsv1.WatchClientsResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]*clientsv1.WatchClientsResponse, len(s.resps))
	copy(out, s.resps)

	return out
}

func waitForSent(t *testing.T, s *fakeWatchSender, n int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(s.snapshot()) >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d responses (got %d)", n, len(s.snapshot()))
}

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

func TestClientsHandler_ListActiveClients(t *testing.T) {
	t.Parallel()

	now := time.Now()
	active := &fakeActiveSource{
		clients: []*domain.Client{
			{ID: "conn-2", PeerAddress: "p2", ConnectedAt: now.Add(time.Second)},
			{ID: "conn-1", PeerAddress: "p1", ConnectedAt: now},
		},
	}
	uc := clientsuc.NewUseCase(active, &fakeHistorySource{})
	h := NewClientsHandler(uc)

	resp, err := h.ListActiveClients(context.Background(), connect.NewRequest(&clientsv1.ListActiveClientsRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetClients(), 2)
	// Sorted by ConnectedAt asc
	assert.Equal(t, "conn-1", resp.Msg.GetClients()[0].GetId())
	assert.Equal(t, "conn-2", resp.Msg.GetClients()[1].GetId())
}

func TestClientsHandler_GetClient_Active(t *testing.T) {
	t.Parallel()

	active := &fakeActiveSource{
		clients: []*domain.Client{{ID: "x", PeerAddress: "p", ConnectedAt: time.Now()}},
		events: map[string][]domain.ClientEvent{
			"x": {{Method: "Put", Key: "/k"}},
		},
	}
	h := NewClientsHandler(clientsuc.NewUseCase(active, &fakeHistorySource{}))

	resp, err := h.GetClient(context.Background(), connect.NewRequest(&clientsv1.GetClientRequest{Id: "x"}))
	require.NoError(t, err)
	assert.Equal(t, "x", resp.Msg.GetClient().GetId())
	require.Len(t, resp.Msg.GetRecentEvents(), 1)
	assert.Equal(t, "Put", resp.Msg.GetRecentEvents()[0].GetMethod())
}

func TestClientsHandler_GetClient_FallbackToHistory(t *testing.T) {
	t.Parallel()

	disconn := time.Now()
	hist := &fakeHistorySource{
		saved: []*domain.Client{
			{ID: "old", PeerAddress: "p", ConnectedAt: disconn.Add(-time.Hour), DisconnectedAt: &disconn},
		},
	}
	active := &fakeActiveSource{}
	h := NewClientsHandler(clientsuc.NewUseCase(active, hist))

	resp, err := h.GetClient(context.Background(), connect.NewRequest(&clientsv1.GetClientRequest{Id: "old"}))
	require.NoError(t, err)
	assert.Equal(t, "old", resp.Msg.GetClient().GetId())
	require.NotNil(t, resp.Msg.GetClient().GetDisconnectedAt())
	assert.Empty(t, resp.Msg.GetRecentEvents(), "history clients have no recent events")
}

func TestClientsHandler_GetClient_NotFound(t *testing.T) {
	t.Parallel()

	h := NewClientsHandler(clientsuc.NewUseCase(&fakeActiveSource{}, &fakeHistorySource{}))

	_, err := h.GetClient(context.Background(), connect.NewRequest(&clientsv1.GetClientRequest{Id: "nope"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestClientsHandler_ListHistoricalConnections(t *testing.T) {
	t.Parallel()

	now := time.Now()
	hist := &fakeHistorySource{saved: []*domain.Client{
		{ID: "a", DisconnectedAt: &now},
		{ID: "b", DisconnectedAt: &now},
	}}
	h := NewClientsHandler(clientsuc.NewUseCase(&fakeActiveSource{}, hist))

	resp, err := h.ListHistoricalConnections(context.Background(),
		connect.NewRequest(&clientsv1.ListHistoricalConnectionsRequest{Limit: 10}))
	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetClients(), 2)
}

func TestClientsHandler_ListHistoricalConnections_PropagatesError(t *testing.T) {
	t.Parallel()

	hist := &fakeHistorySource{listErr: errors.New("db down")}
	h := NewClientsHandler(clientsuc.NewUseCase(&fakeActiveSource{}, hist))

	_, err := h.ListHistoricalConnections(context.Background(),
		connect.NewRequest(&clientsv1.ListHistoricalConnectionsRequest{}))
	require.Error(t, err)
}

// -----------------------------------------------------------------------------
// Watch streaming — memory leak guards
// -----------------------------------------------------------------------------

func TestClientsHandler_runWatch_SendsInitialSnapshot(t *testing.T) {
	t.Parallel()

	now := time.Now()
	active := &fakeActiveSource{
		clients: []*domain.Client{{ID: "conn-1", PeerAddress: "p", ConnectedAt: now}},
	}
	h := NewClientsHandler(clientsuc.NewUseCase(active, &fakeHistorySource{})).
		WithSnapshotInterval(time.Hour) // effectively disable periodic ticks

	sender := &fakeWatchSender{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.runWatch(ctx, sender) }()

	waitForSent(t, sender, 1)

	got := sender.snapshot()[0]
	assert.Equal(t, clientsv1.WatchClientsResponse_KIND_SNAPSHOT, got.GetKind())
	require.Len(t, got.GetClients(), 1)
	assert.Equal(t, "conn-1", got.GetClients()[0].GetId())

	cancel()
	require.NoError(t, <-done)
}

func TestClientsHandler_runWatch_PushesConnectedEvent(t *testing.T) {
	t.Parallel()

	active := &fakeActiveSource{}
	h := NewClientsHandler(clientsuc.NewUseCase(active, &fakeHistorySource{})).
		WithSnapshotInterval(time.Hour)

	sender := &fakeWatchSender{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.runWatch(ctx, sender) }()

	waitForSent(t, sender, 1) // initial snapshot

	// Need to wait for runWatch to have actually subscribed before pushing.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if active.activeSubs() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	active.push(domain.ClientChange{
		Kind:   domain.ClientConnected,
		Client: &domain.Client{ID: "new"},
	})

	waitForSent(t, sender, 2)

	got := sender.snapshot()[1]
	assert.Equal(t, clientsv1.WatchClientsResponse_KIND_CONNECTED, got.GetKind())
	require.Len(t, got.GetClients(), 1)
	assert.Equal(t, "new", got.GetClients()[0].GetId())

	cancel()
	require.NoError(t, <-done)
}

func TestClientsHandler_runWatch_DisconnectedEvent(t *testing.T) {
	t.Parallel()

	active := &fakeActiveSource{}
	h := NewClientsHandler(clientsuc.NewUseCase(active, &fakeHistorySource{})).
		WithSnapshotInterval(time.Hour)

	sender := &fakeWatchSender{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.runWatch(ctx, sender) }()
	waitForSent(t, sender, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if active.activeSubs() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	active.push(domain.ClientChange{
		Kind:   domain.ClientDisconnected,
		Client: &domain.Client{ID: "gone", DisconnectedAt: new(time.Now())},
	})

	waitForSent(t, sender, 2)
	assert.Equal(t, clientsv1.WatchClientsResponse_KIND_DISCONNECTED, sender.snapshot()[1].GetKind())

	cancel()
	require.NoError(t, <-done)
}

func TestClientsHandler_runWatch_ActivityEventsAreSwallowed(t *testing.T) {
	t.Parallel()

	active := &fakeActiveSource{}
	h := NewClientsHandler(clientsuc.NewUseCase(active, &fakeHistorySource{})).
		WithSnapshotInterval(time.Hour)

	sender := &fakeWatchSender{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.runWatch(ctx, sender) }()
	waitForSent(t, sender, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if active.activeSubs() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	for range 5 {
		active.push(domain.ClientChange{Kind: domain.ClientActivity, Client: &domain.Client{ID: "x"}})
	}

	// Give the handler a moment to process — none of these should produce a Send.
	time.Sleep(50 * time.Millisecond)
	assert.Len(t, sender.snapshot(), 1, "Activity events must not be sent over the wire")

	cancel()
	require.NoError(t, <-done)
}

func TestClientsHandler_runWatch_PeriodicSnapshot(t *testing.T) {
	t.Parallel()

	active := &fakeActiveSource{
		clients: []*domain.Client{{ID: "c"}},
	}
	h := NewClientsHandler(clientsuc.NewUseCase(active, &fakeHistorySource{})).
		WithSnapshotInterval(40 * time.Millisecond)

	sender := &fakeWatchSender{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.runWatch(ctx, sender) }()

	waitForSent(t, sender, 3) // initial + ≥2 ticks

	cancel()
	require.NoError(t, <-done)

	for _, r := range sender.snapshot() {
		assert.Equal(t, clientsv1.WatchClientsResponse_KIND_SNAPSHOT, r.GetKind())
	}
}

// LEAK GUARD: ctx cancel must release the subscription.
func TestClientsHandler_runWatch_CtxCancel_UnsubscribesAndExits(t *testing.T) {
	t.Parallel()

	active := &fakeActiveSource{}
	h := NewClientsHandler(clientsuc.NewUseCase(active, &fakeHistorySource{})).
		WithSnapshotInterval(time.Hour)

	sender := &fakeWatchSender{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.runWatch(ctx, sender) }()
	waitForSent(t, sender, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if active.activeSubs() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	require.Equal(t, 1, active.activeSubs(), "subscription opened after subscribe")

	cancel()

	select {
	case err := <-done:
		require.NoError(t, err, "ctx cancel must return nil, not error")
	case <-time.After(time.Second):
		t.Fatal("runWatch did not return after ctx cancel")
	}

	assert.Equal(t, 0, active.activeSubs(), "subscription released on exit")
}

// LEAK GUARD: Send error must release the subscription.
func TestClientsHandler_runWatch_SendError_ReleasesSubscription(t *testing.T) {
	t.Parallel()

	active := &fakeActiveSource{
		clients: []*domain.Client{{ID: "c"}},
	}
	h := NewClientsHandler(clientsuc.NewUseCase(active, &fakeHistorySource{})).
		WithSnapshotInterval(time.Hour)

	sender := &fakeWatchSender{sendErr: errors.New("client closed")}
	ctx := t.Context()

	done := make(chan error, 1)
	go func() { done <- h.runWatch(ctx, sender) }()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("runWatch did not exit on send error")
	}

	assert.Equal(t, 0, active.activeSubs(), "subscription released even on send error")
}

// LEAK GUARD: registry shutdown (subscription channel closes) must exit cleanly.
func TestClientsHandler_runWatch_RegistryShutdown_ExitsCleanly(t *testing.T) {
	t.Parallel()

	active := &fakeActiveSource{}
	h := NewClientsHandler(clientsuc.NewUseCase(active, &fakeHistorySource{})).
		WithSnapshotInterval(time.Hour)

	sender := &fakeWatchSender{}
	ctx := t.Context()

	done := make(chan error, 1)
	go func() { done <- h.runWatch(ctx, sender) }()
	waitForSent(t, sender, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if active.activeSubs() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Simulate registry shutdown by manually closing the subscriber's channel.
	active.subscribersMu.Lock()
	for _, ch := range active.subscribers {
		close(ch)
	}
	active.subscribers = nil
	active.subscribersMu.Unlock()

	select {
	case err := <-done:
		require.NoError(t, err, "channel close must exit cleanly")
	case <-time.After(time.Second):
		t.Fatal("runWatch did not exit when subscription channel closed")
	}
}

// -----------------------------------------------------------------------------
// ListClientSessions
// -----------------------------------------------------------------------------

func TestClientsHandler_ListClientSessions(t *testing.T) {
	t.Parallel()

	d := time.Now()
	hist := &fakeHistorySource{saved: []*domain.Client{
		{ID: "a", ClientName: "order-service", K8sNamespace: "production", DisconnectedAt: &d},
		{ID: "b", ClientName: "order-service", K8sNamespace: "production", DisconnectedAt: &d},
		{ID: "c", ClientName: "order-service", K8sNamespace: "staging", DisconnectedAt: &d},
	}}
	h := NewClientsHandler(clientsuc.NewUseCase(&fakeActiveSource{}, hist))

	resp, err := h.ListClientSessions(context.Background(),
		connect.NewRequest(&clientsv1.ListClientSessionsRequest{
			ClientName:   "order-service",
			K8SNamespace: "production",
		}))
	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetSessions(), 2)
}

func TestClientsHandler_ListClientSessions_ExcludesCurrent(t *testing.T) {
	t.Parallel()

	d := time.Now()
	hist := &fakeHistorySource{saved: []*domain.Client{
		{ID: "a", ClientName: "x", K8sNamespace: "p", DisconnectedAt: &d},
		{ID: "b", ClientName: "x", K8sNamespace: "p", DisconnectedAt: &d},
	}}
	h := NewClientsHandler(clientsuc.NewUseCase(&fakeActiveSource{}, hist))

	resp, err := h.ListClientSessions(context.Background(),
		connect.NewRequest(&clientsv1.ListClientSessionsRequest{
			ClientName:   "x",
			K8SNamespace: "p",
			CurrentId:    "a",
		}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetSessions(), 1)
	assert.Equal(t, "b", resp.Msg.GetSessions()[0].GetId())
}

// -----------------------------------------------------------------------------
// WatchClient streaming — leak guards
// -----------------------------------------------------------------------------

// fakeWatchClientSender captures sent responses for assertions.
type fakeWatchClientSender struct {
	mu      sync.Mutex
	resps   []*clientsv1.WatchClientResponse
	sendErr error
}

func (s *fakeWatchClientSender) Send(r *clientsv1.WatchClientResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sendErr != nil {
		return s.sendErr
	}

	s.resps = append(s.resps, r)

	return nil
}

func (s *fakeWatchClientSender) snapshot() []*clientsv1.WatchClientResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]*clientsv1.WatchClientResponse, len(s.resps))
	copy(out, s.resps)

	return out
}

func waitForClientSent(t *testing.T, s *fakeWatchClientSender, n int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(s.snapshot()) >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d responses (got %d)", n, len(s.snapshot()))
}

func TestClientsHandler_runWatchClient_NotFound(t *testing.T) {
	t.Parallel()

	h := NewClientsHandler(clientsuc.NewUseCase(&fakeActiveSource{}, &fakeHistorySource{}))

	err := h.runWatchClient(context.Background(), "missing", &fakeWatchClientSender{})
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestClientsHandler_runWatchClient_AlreadyDisconnected_SendsSingleFrameAndExits(t *testing.T) {
	t.Parallel()

	active := &fakeActiveSource{}
	hist := &fakeHistorySource{saved: []*domain.Client{
		{ID: "x", DisconnectedAt: new(time.Now()), ClientName: "svc"},
	}}
	h := NewClientsHandler(clientsuc.NewUseCase(active, hist)).WithSnapshotInterval(time.Hour)

	sender := &fakeWatchClientSender{}
	err := h.runWatchClient(context.Background(), "x", sender)
	require.NoError(t, err)

	resps := sender.snapshot()
	require.Len(t, resps, 1)
	assert.Equal(t, clientsv1.WatchClientResponse_KIND_DISCONNECTED, resps[0].GetKind())
}

func TestClientsHandler_runWatchClient_InitialSnapshot(t *testing.T) {
	t.Parallel()

	active := &fakeActiveSource{
		clients: []*domain.Client{{ID: "x", ClientName: "svc", ConnectedAt: time.Now()}},
	}
	h := NewClientsHandler(clientsuc.NewUseCase(active, &fakeHistorySource{})).
		WithSnapshotInterval(time.Hour)

	sender := &fakeWatchClientSender{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.runWatchClient(ctx, "x", sender) }()

	waitForClientSent(t, sender, 1)
	resps := sender.snapshot()
	assert.Equal(t, clientsv1.WatchClientResponse_KIND_SNAPSHOT, resps[0].GetKind())
	assert.Equal(t, "x", resps[0].GetClient().GetId())

	cancel()
	require.NoError(t, <-done)
}

func TestClientsHandler_runWatchClient_ForwardsRequestRecorded(t *testing.T) {
	t.Parallel()

	active := &fakeActiveSource{
		clients: []*domain.Client{{ID: "x", ConnectedAt: time.Now()}},
	}
	h := NewClientsHandler(clientsuc.NewUseCase(active, &fakeHistorySource{})).
		WithSnapshotInterval(time.Hour)

	sender := &fakeWatchClientSender{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.runWatchClient(ctx, "x", sender) }()
	waitForClientSent(t, sender, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if active.activeSubs() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	active.push(domain.ClientChange{
		Kind:   domain.ClientRequestRecorded,
		Client: &domain.Client{ID: "x"},
		Event:  &domain.ClientEvent{Method: "Put", Key: "/k", Timestamp: time.Now()},
	})

	waitForClientSent(t, sender, 2)
	got := sender.snapshot()[1]
	assert.Equal(t, clientsv1.WatchClientResponse_KIND_REQUEST_RECORDED, got.GetKind())
	require.NotNil(t, got.GetEvent())
	assert.Equal(t, "Put", got.GetEvent().GetMethod())

	cancel()
	require.NoError(t, <-done)
}

func TestClientsHandler_runWatchClient_DisconnectExitsCleanly(t *testing.T) {
	t.Parallel()

	active := &fakeActiveSource{clients: []*domain.Client{{ID: "x"}}}
	h := NewClientsHandler(clientsuc.NewUseCase(active, &fakeHistorySource{})).
		WithSnapshotInterval(time.Hour)

	sender := &fakeWatchClientSender{}
	ctx := t.Context()

	done := make(chan error, 1)
	go func() { done <- h.runWatchClient(ctx, "x", sender) }()
	waitForClientSent(t, sender, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if active.activeSubs() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	active.push(domain.ClientChange{
		Kind:   domain.ClientDisconnected,
		Client: &domain.Client{ID: "x", DisconnectedAt: new(time.Now())},
	})

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("runWatchClient did not exit on disconnect")
	}

	resps := sender.snapshot()
	require.Len(t, resps, 2)
	assert.Equal(t, clientsv1.WatchClientResponse_KIND_DISCONNECTED, resps[1].GetKind())
}

// LEAK GUARD: ctx cancel must release subscription.
func TestClientsHandler_runWatchClient_CtxCancel_ReleasesSubscription(t *testing.T) {
	t.Parallel()

	active := &fakeActiveSource{clients: []*domain.Client{{ID: "x"}}}
	h := NewClientsHandler(clientsuc.NewUseCase(active, &fakeHistorySource{})).
		WithSnapshotInterval(time.Hour)

	sender := &fakeWatchClientSender{}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- h.runWatchClient(ctx, "x", sender) }()
	waitForClientSent(t, sender, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if active.activeSubs() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	require.Equal(t, 1, active.activeSubs())

	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("runWatchClient did not exit on ctx cancel")
	}

	assert.Equal(t, 0, active.activeSubs(), "subscription released on ctx cancel")
}

// LEAK GUARD: send error must release subscription.
func TestClientsHandler_runWatchClient_SendError_ReleasesSubscription(t *testing.T) {
	t.Parallel()

	active := &fakeActiveSource{clients: []*domain.Client{{ID: "x"}}}
	h := NewClientsHandler(clientsuc.NewUseCase(active, &fakeHistorySource{})).
		WithSnapshotInterval(time.Hour)

	sender := &fakeWatchClientSender{sendErr: errors.New("client closed")}
	ctx := t.Context()

	done := make(chan error, 1)
	go func() { done <- h.runWatchClient(ctx, "x", sender) }()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("runWatchClient did not exit on send error")
	}

	assert.Equal(t, 0, active.activeSubs(), "subscription released after send error")
}

func TestClientsHandler_runWatchClient_PeriodicSnapshot(t *testing.T) {
	t.Parallel()

	active := &fakeActiveSource{clients: []*domain.Client{{ID: "x"}}}
	h := NewClientsHandler(clientsuc.NewUseCase(active, &fakeHistorySource{})).
		WithSnapshotInterval(40 * time.Millisecond)

	sender := &fakeWatchClientSender{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.runWatchClient(ctx, "x", sender) }()

	waitForClientSent(t, sender, 3) // initial + ≥2 ticks

	cancel()
	require.NoError(t, <-done)

	for _, r := range sender.snapshot() {
		assert.Equal(t, clientsv1.WatchClientResponse_KIND_SNAPSHOT, r.GetKind())
	}
}

func TestClientsHandler_runWatch_SubscribeOnlyOnce(t *testing.T) {
	t.Parallel()

	// Defensive: each runWatch must Subscribe exactly once and unsubscribe exactly once.
	active := &fakeActiveSource{}
	h := NewClientsHandler(clientsuc.NewUseCase(active, &fakeHistorySource{})).
		WithSnapshotInterval(time.Hour)

	sender := &fakeWatchSender{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.runWatch(ctx, sender) }()
	waitForSent(t, sender, 1)

	cancel()
	<-done

	assert.Equal(t, 1, active.subscribeCalls)
	assert.Equal(t, 1, active.subscribeCancel)
}
