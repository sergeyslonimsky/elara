package clients

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	clients_mock "github.com/sergeyslonimsky/elara/internal/handler/v2/clients/mocks"
	clientsv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/clients/v1"
)

// fakeUsecase implements the handler's usecase interface for streaming tests
// where gomock with channels is awkward. Captures subscription lifecycle.
type fakeUsecase struct {
	mu sync.Mutex

	activeClients []*domain.Client
	getClient     *domain.Client
	getEvents     []domain.ClientEvent
	getErr        error

	subscribersMu   sync.Mutex
	subscribers     []chan domain.ClientChange
	subscribeCalls  int
	subscribeCancel int
}

func (f *fakeUsecase) ListActive(_ context.Context) ([]*domain.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*domain.Client, len(f.activeClients))
	copy(out, f.activeClients)

	return out, nil
}

func (f *fakeUsecase) ListHistorical(_ context.Context, _ int) ([]*domain.Client, error) {
	return nil, nil
}

func (f *fakeUsecase) ListSessions(_ context.Context, _, _, _ string, _ int) ([]*domain.Client, error) {
	return nil, nil
}

func (f *fakeUsecase) Get(_ context.Context, _ string) (*domain.Client, []domain.ClientEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.getClient, f.getEvents, f.getErr
}

func (f *fakeUsecase) SubscribeChanges(_ context.Context) (<-chan domain.ClientChange, func(), error) {
	return f.subscribe()
}

func (f *fakeUsecase) SubscribeClient(_ context.Context, _ string) (<-chan domain.ClientChange, func(), error) {
	return f.subscribe()
}

func (f *fakeUsecase) subscribe() (<-chan domain.ClientChange, func(), error) {
	f.subscribersMu.Lock()
	defer f.subscribersMu.Unlock()

	f.subscribeCalls++
	ch := make(chan domain.ClientChange, 8)
	f.subscribers = append(f.subscribers, ch)

	cleanup := func() {
		f.subscribersMu.Lock()
		defer f.subscribersMu.Unlock()
		for i, c := range f.subscribers {
			if c == ch {
				f.subscribers = append(f.subscribers[:i], f.subscribers[i+1:]...)
				close(ch)
				f.subscribeCancel++

				return
			}
		}
	}

	return ch, cleanup, nil
}

func (f *fakeUsecase) push(ev domain.ClientChange) {
	f.subscribersMu.Lock()
	defer f.subscribersMu.Unlock()
	for _, c := range f.subscribers {
		select {
		case c <- ev:
		default:
		}
	}
}

func (f *fakeUsecase) activeSubs() int {
	f.subscribersMu.Lock()
	defer f.subscribersMu.Unlock()

	return len(f.subscribers)
}

func (f *fakeUsecase) setActiveClients(cs []*domain.Client) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.activeClients = cs
}

func (f *fakeUsecase) setGet(c *domain.Client, events []domain.ClientEvent, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getClient = c
	f.getEvents = events
	f.getErr = err
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
// Tests using gomock for non-streaming methods
// -----------------------------------------------------------------------------

func TestClientsHandler_ListActiveClients(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := clients_mock.NewMockusecase(ctrl)

	now := time.Now()
	uc.EXPECT().ListActive(gomock.Any()).Return([]*domain.Client{
		{ID: "conn-1", PeerAddress: "p1", ConnectedAt: now},
		{ID: "conn-2", PeerAddress: "p2", ConnectedAt: now.Add(time.Second)},
	}, nil)

	h := New(uc)
	resp, err := h.ListActiveClients(t.Context(), connect.NewRequest(&clientsv1.ListActiveClientsRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetClients(), 2)
	assert.Equal(t, "conn-1", resp.Msg.GetClients()[0].GetId())
	assert.Equal(t, "conn-2", resp.Msg.GetClients()[1].GetId())
}

func TestClientsHandler_GetClient_Active(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := clients_mock.NewMockusecase(ctrl)
	uc.EXPECT().Get(gomock.Any(), "x").Return(
		&domain.Client{ID: "x", PeerAddress: "p", ConnectedAt: time.Now()},
		[]domain.ClientEvent{{Method: "Put", Key: "/k"}},
		nil,
	)

	h := New(uc)
	resp, err := h.GetClient(t.Context(), connect.NewRequest(&clientsv1.GetClientRequest{Id: "x"}))
	require.NoError(t, err)
	assert.Equal(t, "x", resp.Msg.GetClient().GetId())
	require.Len(t, resp.Msg.GetRecentEvents(), 1)
	assert.Equal(t, "Put", resp.Msg.GetRecentEvents()[0].GetMethod())
}

func TestClientsHandler_GetClient_FallbackToHistory(t *testing.T) {
	t.Parallel()

	disconn := time.Now()
	ctrl := gomock.NewController(t)
	uc := clients_mock.NewMockusecase(ctrl)
	uc.EXPECT().Get(gomock.Any(), "old").Return(
		&domain.Client{ID: "old", PeerAddress: "p", ConnectedAt: disconn.Add(-time.Hour), DisconnectedAt: &disconn},
		nil,
		nil,
	)

	h := New(uc)
	resp, err := h.GetClient(t.Context(), connect.NewRequest(&clientsv1.GetClientRequest{Id: "old"}))
	require.NoError(t, err)
	assert.Equal(t, "old", resp.Msg.GetClient().GetId())
	require.NotNil(t, resp.Msg.GetClient().GetDisconnectedAt())
	assert.Empty(t, resp.Msg.GetRecentEvents())
}

func TestClientsHandler_GetClient_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := clients_mock.NewMockusecase(ctrl)
	uc.EXPECT().Get(gomock.Any(), "nope").Return(nil, nil, nil)

	h := New(uc)
	_, err := h.GetClient(t.Context(), connect.NewRequest(&clientsv1.GetClientRequest{Id: "nope"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestClientsHandler_ListHistoricalConnections(t *testing.T) {
	t.Parallel()

	now := time.Now()
	ctrl := gomock.NewController(t)
	uc := clients_mock.NewMockusecase(ctrl)
	uc.EXPECT().ListHistorical(gomock.Any(), 10).Return([]*domain.Client{
		{ID: "a", DisconnectedAt: &now},
		{ID: "b", DisconnectedAt: &now},
	}, nil)

	h := New(uc)
	resp, err := h.ListHistoricalConnections(t.Context(),
		connect.NewRequest(&clientsv1.ListHistoricalConnectionsRequest{Limit: 10}))
	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetClients(), 2)
}

func TestClientsHandler_ListHistoricalConnections_PropagatesError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := clients_mock.NewMockusecase(ctrl)
	uc.EXPECT().ListHistorical(gomock.Any(), gomock.Any()).Return(nil, errors.New("db down"))

	h := New(uc)
	_, err := h.ListHistoricalConnections(t.Context(),
		connect.NewRequest(&clientsv1.ListHistoricalConnectionsRequest{}))
	require.Error(t, err)
}

// -----------------------------------------------------------------------------
// ListClientSessions
// -----------------------------------------------------------------------------

func TestClientsHandler_ListClientSessions(t *testing.T) {
	t.Parallel()

	d := time.Now()
	ctrl := gomock.NewController(t)
	uc := clients_mock.NewMockusecase(ctrl)
	uc.EXPECT().
		ListSessions(gomock.Any(), "order-service", "production", "", 0).
		Return([]*domain.Client{
			{ID: "a", ClientName: "order-service", K8sNamespace: "production", DisconnectedAt: &d},
			{ID: "b", ClientName: "order-service", K8sNamespace: "production", DisconnectedAt: &d},
		}, nil)

	h := New(uc)
	resp, err := h.ListClientSessions(t.Context(),
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
	ctrl := gomock.NewController(t)
	uc := clients_mock.NewMockusecase(ctrl)
	uc.EXPECT().
		ListSessions(gomock.Any(), "x", "p", "a", 0).
		Return([]*domain.Client{
			{ID: "b", ClientName: "x", K8sNamespace: "p", DisconnectedAt: &d},
		}, nil)

	h := New(uc)
	resp, err := h.ListClientSessions(t.Context(),
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
// Watch streaming — uses fakeUsecase (channel-based subscription is awkward in gomock)
// -----------------------------------------------------------------------------

func TestClientsHandler_runWatch_SendsInitialSnapshot(t *testing.T) {
	t.Parallel()

	now := time.Now()
	uc := &fakeUsecase{}
	uc.setActiveClients([]*domain.Client{{ID: "conn-1", PeerAddress: "p", ConnectedAt: now}})

	h := New(uc).WithSnapshotInterval(time.Hour)

	sender := &fakeWatchSender{}
	ctx, cancel := context.WithCancel(t.Context())
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

	uc := &fakeUsecase{}
	h := New(uc).WithSnapshotInterval(time.Hour)

	sender := &fakeWatchSender{}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- h.runWatch(ctx, sender) }()

	waitForSent(t, sender, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if uc.activeSubs() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	uc.push(domain.ClientChange{
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

	uc := &fakeUsecase{}
	h := New(uc).WithSnapshotInterval(time.Hour)

	sender := &fakeWatchSender{}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- h.runWatch(ctx, sender) }()
	waitForSent(t, sender, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if uc.activeSubs() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	now := time.Now()
	uc.push(domain.ClientChange{
		Kind:   domain.ClientDisconnected,
		Client: &domain.Client{ID: "gone", DisconnectedAt: &now},
	})

	waitForSent(t, sender, 2)
	assert.Equal(t, clientsv1.WatchClientsResponse_KIND_DISCONNECTED, sender.snapshot()[1].GetKind())

	cancel()
	require.NoError(t, <-done)
}

func TestClientsHandler_runWatch_ActivityEventsAreSwallowed(t *testing.T) {
	t.Parallel()

	uc := &fakeUsecase{}
	h := New(uc).WithSnapshotInterval(time.Hour)

	sender := &fakeWatchSender{}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- h.runWatch(ctx, sender) }()
	waitForSent(t, sender, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if uc.activeSubs() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	for range 5 {
		uc.push(domain.ClientChange{Kind: domain.ClientActivity, Client: &domain.Client{ID: "x"}})
	}

	time.Sleep(50 * time.Millisecond)
	assert.Len(t, sender.snapshot(), 1, "Activity events must not be sent over the wire")

	cancel()
	require.NoError(t, <-done)
}

func TestClientsHandler_runWatch_PeriodicSnapshot(t *testing.T) {
	t.Parallel()

	uc := &fakeUsecase{}
	uc.setActiveClients([]*domain.Client{{ID: "c"}})
	h := New(uc).WithSnapshotInterval(40 * time.Millisecond)

	sender := &fakeWatchSender{}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- h.runWatch(ctx, sender) }()

	waitForSent(t, sender, 3)

	cancel()
	require.NoError(t, <-done)

	for _, r := range sender.snapshot() {
		assert.Equal(t, clientsv1.WatchClientsResponse_KIND_SNAPSHOT, r.GetKind())
	}
}

func TestClientsHandler_runWatch_CtxCancel_UnsubscribesAndExits(t *testing.T) {
	t.Parallel()

	uc := &fakeUsecase{}
	h := New(uc).WithSnapshotInterval(time.Hour)

	sender := &fakeWatchSender{}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- h.runWatch(ctx, sender) }()
	waitForSent(t, sender, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if uc.activeSubs() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	require.Equal(t, 1, uc.activeSubs())

	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("runWatch did not return after ctx cancel")
	}

	assert.Equal(t, 0, uc.activeSubs())
}

func TestClientsHandler_runWatch_SendError_ReleasesSubscription(t *testing.T) {
	t.Parallel()

	uc := &fakeUsecase{}
	uc.setActiveClients([]*domain.Client{{ID: "c"}})
	h := New(uc).WithSnapshotInterval(time.Hour)

	sender := &fakeWatchSender{sendErr: errors.New("client closed")}

	done := make(chan error, 1)
	go func() { done <- h.runWatch(t.Context(), sender) }()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("runWatch did not exit on send error")
	}

	assert.Equal(t, 0, uc.activeSubs())
}

func TestClientsHandler_runWatch_RegistryShutdown_ExitsCleanly(t *testing.T) {
	t.Parallel()

	uc := &fakeUsecase{}
	h := New(uc).WithSnapshotInterval(time.Hour)

	sender := &fakeWatchSender{}

	done := make(chan error, 1)
	go func() { done <- h.runWatch(t.Context(), sender) }()
	waitForSent(t, sender, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if uc.activeSubs() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	uc.subscribersMu.Lock()
	for _, ch := range uc.subscribers {
		close(ch)
	}
	uc.subscribers = nil
	uc.subscribersMu.Unlock()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("runWatch did not exit when subscription channel closed")
	}
}

func TestClientsHandler_runWatch_SubscribeOnlyOnce(t *testing.T) {
	t.Parallel()

	uc := &fakeUsecase{}
	h := New(uc).WithSnapshotInterval(time.Hour)

	sender := &fakeWatchSender{}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- h.runWatch(ctx, sender) }()
	waitForSent(t, sender, 1)

	cancel()
	<-done

	assert.Equal(t, 1, uc.subscribeCalls)
	assert.Equal(t, 1, uc.subscribeCancel)
}

// -----------------------------------------------------------------------------
// WatchClient streaming
// -----------------------------------------------------------------------------

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

	uc := &fakeUsecase{}
	uc.setGet(nil, nil, nil)

	h := New(uc)
	err := h.runWatchClient(t.Context(), "missing", &fakeWatchClientSender{})
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestClientsHandler_runWatchClient_AlreadyDisconnected_SendsSingleFrameAndExits(t *testing.T) {
	t.Parallel()

	now := time.Now()
	uc := &fakeUsecase{}
	uc.setGet(&domain.Client{ID: "x", DisconnectedAt: &now, ClientName: "svc"}, nil, nil)

	h := New(uc).WithSnapshotInterval(time.Hour)

	sender := &fakeWatchClientSender{}
	err := h.runWatchClient(t.Context(), "x", sender)
	require.NoError(t, err)

	resps := sender.snapshot()
	require.Len(t, resps, 1)
	assert.Equal(t, clientsv1.WatchClientResponse_KIND_DISCONNECTED, resps[0].GetKind())
}

func TestClientsHandler_runWatchClient_InitialSnapshot(t *testing.T) {
	t.Parallel()

	uc := &fakeUsecase{}
	uc.setGet(&domain.Client{ID: "x", ClientName: "svc", ConnectedAt: time.Now()}, nil, nil)

	h := New(uc).WithSnapshotInterval(time.Hour)

	sender := &fakeWatchClientSender{}
	ctx, cancel := context.WithCancel(t.Context())
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

	uc := &fakeUsecase{}
	uc.setGet(&domain.Client{ID: "x", ConnectedAt: time.Now()}, nil, nil)

	h := New(uc).WithSnapshotInterval(time.Hour)

	sender := &fakeWatchClientSender{}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- h.runWatchClient(ctx, "x", sender) }()
	waitForClientSent(t, sender, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if uc.activeSubs() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	uc.push(domain.ClientChange{
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

	uc := &fakeUsecase{}
	uc.setGet(&domain.Client{ID: "x", ConnectedAt: time.Now()}, nil, nil)

	h := New(uc).WithSnapshotInterval(time.Hour)

	sender := &fakeWatchClientSender{}

	done := make(chan error, 1)
	go func() { done <- h.runWatchClient(t.Context(), "x", sender) }()
	waitForClientSent(t, sender, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if uc.activeSubs() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	now := time.Now()
	uc.push(domain.ClientChange{
		Kind:   domain.ClientDisconnected,
		Client: &domain.Client{ID: "x", DisconnectedAt: &now},
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

func TestClientsHandler_runWatchClient_CtxCancel_ReleasesSubscription(t *testing.T) {
	t.Parallel()

	uc := &fakeUsecase{}
	uc.setGet(&domain.Client{ID: "x", ConnectedAt: time.Now()}, nil, nil)

	h := New(uc).WithSnapshotInterval(time.Hour)

	sender := &fakeWatchClientSender{}
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan error, 1)
	go func() { done <- h.runWatchClient(ctx, "x", sender) }()
	waitForClientSent(t, sender, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if uc.activeSubs() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	require.Equal(t, 1, uc.activeSubs())

	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("runWatchClient did not exit on ctx cancel")
	}

	assert.Equal(t, 0, uc.activeSubs())
}

func TestClientsHandler_runWatchClient_SendError_ReleasesSubscription(t *testing.T) {
	t.Parallel()

	uc := &fakeUsecase{}
	uc.setGet(&domain.Client{ID: "x", ConnectedAt: time.Now()}, nil, nil)

	h := New(uc).WithSnapshotInterval(time.Hour)

	sender := &fakeWatchClientSender{sendErr: errors.New("client closed")}

	done := make(chan error, 1)
	go func() { done <- h.runWatchClient(t.Context(), "x", sender) }()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("runWatchClient did not exit on send error")
	}

	assert.Equal(t, 0, uc.activeSubs())
}

func TestClientsHandler_runWatchClient_PeriodicSnapshot(t *testing.T) {
	t.Parallel()

	uc := &fakeUsecase{}
	uc.setGet(&domain.Client{ID: "x", ConnectedAt: time.Now()}, nil, nil)

	h := New(uc).WithSnapshotInterval(40 * time.Millisecond)

	sender := &fakeWatchClientSender{}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- h.runWatchClient(ctx, "x", sender) }()

	waitForClientSent(t, sender, 3)

	cancel()
	require.NoError(t, <-done)

	for _, r := range sender.snapshot() {
		assert.Equal(t, clientsv1.WatchClientResponse_KIND_SNAPSHOT, r.GetKind())
	}
}
