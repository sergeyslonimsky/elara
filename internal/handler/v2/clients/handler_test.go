package clients_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/clients"
	clientsmock "github.com/sergeyslonimsky/elara/internal/handler/v2/clients/mocks"
	clientsv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/clients/v1"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/clients/v1/clientsv1connect"
)

// -----------------------------------------------------------------------------
// Test fixtures
// -----------------------------------------------------------------------------

// streamFixture controls a single subscription channel + tracks unsubscribe
// calls. Returned by setupSubscribeChanges / setupSubscribeClient — the test
// pushes events through ch, asserts on cancelled to verify cleanup semantics.
type streamFixture struct {
	ch        chan domain.ClientChange
	cancelled atomic.Int32
}

func (s *streamFixture) push(t *testing.T, ev domain.ClientChange) {
	t.Helper()
	select {
	case s.ch <- ev:
	case <-time.After(time.Second):
		t.Fatal("timed out pushing event into subscription channel")
	}
}

// waitForCancel polls until the handler has invoked the cleanup callback.
// Used by tests that assert leak-safety on shutdown / ctx-cancel / send-error.
func (s *streamFixture) waitForCancel(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if s.cancelled.Load() > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("subscription cleanup was never called")
}

// setupSubscribeChanges wires gomock.EXPECT().SubscribeChanges(...) to return a
// fresh streamFixture. The cleanup callback increments cancelled (no close —
// the channel close is reserved for "registry shutdown" simulation tests).
func setupSubscribeChanges(uc *clientsmock.Mockusecase) *streamFixture {
	fx := &streamFixture{ch: make(chan domain.ClientChange, 8)}
	uc.EXPECT().SubscribeChanges(gomock.Any()).
		DoAndReturn(func(_ context.Context) (<-chan domain.ClientChange, func(), error) {
			return fx.ch, func() { fx.cancelled.Add(1) }, nil
		})

	return fx
}

func setupSubscribeClient(uc *clientsmock.Mockusecase, id string) *streamFixture {
	fx := &streamFixture{ch: make(chan domain.ClientChange, 8)}
	uc.EXPECT().SubscribeClient(gomock.Any(), id).
		DoAndReturn(func(_ context.Context, _ string) (<-chan domain.ClientChange, func(), error) {
			return fx.ch, func() { fx.cancelled.Add(1) }, nil
		})

	return fx
}

// newTestServer mounts the handler on a real ConnectRPC route over HTTP/1.1
// (Connect protocol supports server-streaming over chunked encoding, no HTTP/2
// required). Returns the server URL; httptest.Server is cleaned up via t.Cleanup.
func newTestServer(t *testing.T, h clientsv1connect.ClientsServiceHandler) string {
	t.Helper()

	mux := http.NewServeMux()
	path, handler := clientsv1connect.NewClientsServiceHandler(h)
	mux.Handle(path, handler)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv.URL
}

func newClient(url string) clientsv1connect.ClientsServiceClient {
	return clientsv1connect.NewClientsServiceClient(http.DefaultClient, url)
}

// -----------------------------------------------------------------------------
// Unary RPCs — direct handler call is already a blackbox API
// -----------------------------------------------------------------------------

func TestClientsHandler_ListActiveClients(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := clientsmock.NewMockusecase(ctrl)

	now := time.Now()
	uc.EXPECT().ListActive(gomock.Any()).Return([]*domain.Client{
		{ID: "conn-1", PeerAddress: "p1", ConnectedAt: now},
		{ID: "conn-2", PeerAddress: "p2", ConnectedAt: now.Add(time.Second)},
	}, nil)

	h := clients.NewHandler(uc)
	resp, err := h.ListActiveClients(t.Context(), connect.NewRequest(&clientsv1.ListActiveClientsRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetClients(), 2)
	assert.Equal(t, "conn-1", resp.Msg.GetClients()[0].GetId())
	assert.Equal(t, "conn-2", resp.Msg.GetClients()[1].GetId())
}

func TestClientsHandler_GetClient(t *testing.T) {
	t.Parallel()

	now := time.Now()
	disconn := now.Add(-time.Hour)

	tests := []struct {
		name       string
		setupMock  func(*clientsmock.Mockusecase)
		reqID      string
		wantErr    bool
		wantCode   connect.Code
		assertResp func(*testing.T, *clientsv1.GetClientResponse)
	}{
		{
			name: "active with recent events",
			setupMock: func(uc *clientsmock.Mockusecase) {
				uc.EXPECT().Get(gomock.Any(), "x").Return(
					&domain.Client{ID: "x", PeerAddress: "p", ConnectedAt: now},
					[]domain.ClientEvent{{Method: "Put", Key: "/k"}},
					nil,
				)
			},
			reqID: "x",
			assertResp: func(t *testing.T, resp *clientsv1.GetClientResponse) {
				t.Helper()
				assert.Equal(t, "x", resp.GetClient().GetId())
				require.Len(t, resp.GetRecentEvents(), 1)
				assert.Equal(t, "Put", resp.GetRecentEvents()[0].GetMethod())
			},
		},
		{
			name: "fallback to history (disconnected)",
			setupMock: func(uc *clientsmock.Mockusecase) {
				uc.EXPECT().Get(gomock.Any(), "old").Return(
					&domain.Client{ID: "old", PeerAddress: "p", ConnectedAt: disconn, DisconnectedAt: &now},
					nil,
					nil,
				)
			},
			reqID: "old",
			assertResp: func(t *testing.T, resp *clientsv1.GetClientResponse) {
				t.Helper()
				assert.Equal(t, "old", resp.GetClient().GetId())
				require.NotNil(t, resp.GetClient().GetDisconnectedAt())
				assert.Empty(t, resp.GetRecentEvents())
			},
		},
		{
			name: "not found",
			setupMock: func(uc *clientsmock.Mockusecase) {
				uc.EXPECT().Get(gomock.Any(), "nope").Return(nil, nil, nil)
			},
			reqID:    "nope",
			wantErr:  true,
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := clientsmock.NewMockusecase(ctrl)
			tc.setupMock(uc)

			h := clients.NewHandler(uc)
			resp, err := h.GetClient(t.Context(), connect.NewRequest(&clientsv1.GetClientRequest{Id: tc.reqID}))

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			tc.assertResp(t, resp.Msg)
		})
	}
}

func TestClientsHandler_ListHistoricalConnections(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name      string
		setupMock func(*clientsmock.Mockusecase)
		wantErr   bool
		wantLen   int
	}{
		{
			name: "success",
			setupMock: func(uc *clientsmock.Mockusecase) {
				uc.EXPECT().ListHistorical(gomock.Any(), 10).Return([]*domain.Client{
					{ID: "a", DisconnectedAt: &now},
					{ID: "b", DisconnectedAt: &now},
				}, nil)
			},
			wantLen: 2,
		},
		{
			name: "propagates error",
			setupMock: func(uc *clientsmock.Mockusecase) {
				uc.EXPECT().ListHistorical(gomock.Any(), gomock.Any()).Return(nil, errors.New("db down"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := clientsmock.NewMockusecase(ctrl)
			tc.setupMock(uc)

			h := clients.NewHandler(uc)
			resp, err := h.ListHistoricalConnections(t.Context(),
				connect.NewRequest(&clientsv1.ListHistoricalConnectionsRequest{Limit: 10}))

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Len(t, resp.Msg.GetClients(), tc.wantLen)
		})
	}
}

func TestClientsHandler_ListClientSessions(t *testing.T) {
	t.Parallel()

	d := time.Now()

	tests := []struct {
		name      string
		req       *clientsv1.ListClientSessionsRequest
		setupMock func(*clientsmock.Mockusecase)
		wantIDs   []string
	}{
		{
			name: "by name and namespace",
			req: &clientsv1.ListClientSessionsRequest{
				ClientName:   "order-service",
				K8SNamespace: "production",
			},
			setupMock: func(uc *clientsmock.Mockusecase) {
				uc.EXPECT().
					ListSessions(gomock.Any(), "order-service", "production", "", 0).
					Return([]*domain.Client{
						{ID: "a", ClientName: "order-service", K8sNamespace: "production", DisconnectedAt: &d},
						{ID: "b", ClientName: "order-service", K8sNamespace: "production", DisconnectedAt: &d},
					}, nil)
			},
			wantIDs: []string{"a", "b"},
		},
		{
			name: "excludes current",
			req: &clientsv1.ListClientSessionsRequest{
				ClientName:   "x",
				K8SNamespace: "p",
				CurrentId:    "a",
			},
			setupMock: func(uc *clientsmock.Mockusecase) {
				uc.EXPECT().
					ListSessions(gomock.Any(), "x", "p", "a", 0).
					Return([]*domain.Client{
						{ID: "b", ClientName: "x", K8sNamespace: "p", DisconnectedAt: &d},
					}, nil)
			},
			wantIDs: []string{"b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := clientsmock.NewMockusecase(ctrl)
			tc.setupMock(uc)

			h := clients.NewHandler(uc)
			resp, err := h.ListClientSessions(t.Context(), connect.NewRequest(tc.req))
			require.NoError(t, err)

			gotIDs := make([]string, 0, len(resp.Msg.GetSessions()))
			for _, s := range resp.Msg.GetSessions() {
				gotIDs = append(gotIDs, s.GetId())
			}
			assert.Equal(t, tc.wantIDs, gotIDs)
		})
	}
}

// -----------------------------------------------------------------------------
// Streaming: WatchClients via real Connect server
// -----------------------------------------------------------------------------

// receiveFrame consumes one frame from the stream with a timeout. Fails the
// test if no frame arrives in time, so tests don't hang on regressions.
func receiveFrame[T any](t *testing.T, stream *connect.ServerStreamForClient[T]) *T {
	t.Helper()

	done := make(chan struct{})
	var ok bool

	go func() {
		ok = stream.Receive()
		close(done)
	}()

	select {
	case <-done:
		require.True(t, ok, "stream.Receive returned false: %v", stream.Err())

		return stream.Msg()
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for stream frame")

		return nil
	}
}

func TestClientsHandler_WatchClients_SendsInitialSnapshot(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := clientsmock.NewMockusecase(ctrl)

	uc.EXPECT().ListActive(gomock.Any()).Return([]*domain.Client{
		{ID: "conn-1", PeerAddress: "p", ConnectedAt: time.Now()},
	}, nil).AnyTimes()
	fx := setupSubscribeChanges(uc)

	url := newTestServer(t, clients.NewHandler(uc, clients.WithSnapshotInterval(time.Hour)))
	stream, err := newClient(url).WatchClients(t.Context(),
		connect.NewRequest(&clientsv1.WatchClientsRequest{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	got := receiveFrame(t, stream)
	assert.Equal(t, clientsv1.WatchClientsResponse_KIND_SNAPSHOT, got.GetKind())
	require.Len(t, got.GetClients(), 1)
	assert.Equal(t, "conn-1", got.GetClients()[0].GetId())

	require.NoError(t, stream.Close())
	fx.waitForCancel(t)
}

func TestClientsHandler_WatchClients_PushesConnectedEvent(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := clientsmock.NewMockusecase(ctrl)

	uc.EXPECT().ListActive(gomock.Any()).Return(nil, nil).AnyTimes()
	fx := setupSubscribeChanges(uc)

	url := newTestServer(t, clients.NewHandler(uc, clients.WithSnapshotInterval(time.Hour)))
	stream, err := newClient(url).WatchClients(t.Context(),
		connect.NewRequest(&clientsv1.WatchClientsRequest{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	_ = receiveFrame(t, stream) // initial snapshot

	fx.push(t, domain.ClientChange{
		Kind:   domain.ClientConnected,
		Client: &domain.Client{ID: "new"},
	})

	got := receiveFrame(t, stream)
	assert.Equal(t, clientsv1.WatchClientsResponse_KIND_CONNECTED, got.GetKind())
	require.Len(t, got.GetClients(), 1)
	assert.Equal(t, "new", got.GetClients()[0].GetId())
}

func TestClientsHandler_WatchClients_PushesDisconnectedEvent(t *testing.T) {
	t.Parallel()

	disconn := time.Now()

	ctrl := gomock.NewController(t)
	uc := clientsmock.NewMockusecase(ctrl)
	uc.EXPECT().ListActive(gomock.Any()).Return(nil, nil).AnyTimes()
	fx := setupSubscribeChanges(uc)

	url := newTestServer(t, clients.NewHandler(uc, clients.WithSnapshotInterval(time.Hour)))
	stream, err := newClient(url).WatchClients(t.Context(),
		connect.NewRequest(&clientsv1.WatchClientsRequest{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	_ = receiveFrame(t, stream)

	fx.push(t, domain.ClientChange{
		Kind:   domain.ClientDisconnected,
		Client: &domain.Client{ID: "gone", DisconnectedAt: &disconn},
	})

	got := receiveFrame(t, stream)
	assert.Equal(t, clientsv1.WatchClientsResponse_KIND_DISCONNECTED, got.GetKind())
}

func TestClientsHandler_WatchClients_ActivityEventsAreSwallowed(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := clientsmock.NewMockusecase(ctrl)

	// One ListActive for the initial snapshot. Activity events must NOT trigger
	// additional ListActive calls.
	uc.EXPECT().ListActive(gomock.Any()).Return(nil, nil)
	fx := setupSubscribeChanges(uc)

	url := newTestServer(t, clients.NewHandler(uc, clients.WithSnapshotInterval(time.Hour)))
	stream, err := newClient(url).WatchClients(t.Context(),
		connect.NewRequest(&clientsv1.WatchClientsRequest{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	_ = receiveFrame(t, stream)

	for range 5 {
		fx.push(t, domain.ClientChange{Kind: domain.ClientActivity, Client: &domain.Client{ID: "x"}})
	}

	// Give the handler 50ms to (mis)handle the activity events. If they were
	// forwarded, the next Receive would unblock before the deadline.
	done := make(chan bool, 1)
	go func() { done <- stream.Receive() }()

	select {
	case got := <-done:
		t.Fatalf("activity event leaked to wire (Receive returned %v, msg=%+v)", got, stream.Msg())
	case <-time.After(50 * time.Millisecond):
	}
}

func TestClientsHandler_WatchClients_PeriodicSnapshot(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := clientsmock.NewMockusecase(ctrl)
	uc.EXPECT().ListActive(gomock.Any()).Return([]*domain.Client{{ID: "c"}}, nil).AnyTimes()
	setupSubscribeChanges(uc)

	url := newTestServer(t, clients.NewHandler(uc, clients.WithSnapshotInterval(40*time.Millisecond)))
	stream, err := newClient(url).WatchClients(t.Context(),
		connect.NewRequest(&clientsv1.WatchClientsRequest{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	// Three SNAPSHOT frames: 1 initial + 2 ticker.
	for range 3 {
		got := receiveFrame(t, stream)
		assert.Equal(t, clientsv1.WatchClientsResponse_KIND_SNAPSHOT, got.GetKind())
	}
}

func TestClientsHandler_WatchClients_CtxCancelReleasesSubscription(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := clientsmock.NewMockusecase(ctrl)
	uc.EXPECT().ListActive(gomock.Any()).Return(nil, nil).AnyTimes()
	fx := setupSubscribeChanges(uc)

	url := newTestServer(t, clients.NewHandler(uc, clients.WithSnapshotInterval(time.Hour)))

	ctx, cancel := context.WithCancel(t.Context())
	stream, err := newClient(url).WatchClients(ctx, connect.NewRequest(&clientsv1.WatchClientsRequest{}))
	require.NoError(t, err)

	_ = receiveFrame(t, stream)
	cancel()

	fx.waitForCancel(t)
}

func TestClientsHandler_WatchClients_RegistryShutdownExitsCleanly(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := clientsmock.NewMockusecase(ctrl)
	uc.EXPECT().ListActive(gomock.Any()).Return(nil, nil).AnyTimes()
	fx := setupSubscribeChanges(uc)

	url := newTestServer(t, clients.NewHandler(uc, clients.WithSnapshotInterval(time.Hour)))
	stream, err := newClient(url).WatchClients(t.Context(),
		connect.NewRequest(&clientsv1.WatchClientsRequest{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	_ = receiveFrame(t, stream)

	// Simulate the publisher closing the subscription channel (e.g. on registry
	// shutdown). The handler must observe channel-closed and exit the stream.
	close(fx.ch)

	// Client-side: Receive returns false (stream ended) with no error.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !stream.Receive() {
			break
		}
	}
	require.NoError(t, stream.Err())
	fx.waitForCancel(t)
}

// -----------------------------------------------------------------------------
// Streaming: WatchClient
// -----------------------------------------------------------------------------

func TestClientsHandler_WatchClient_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := clientsmock.NewMockusecase(ctrl)
	uc.EXPECT().Get(gomock.Any(), "missing").Return(nil, nil, nil)

	url := newTestServer(t, clients.NewHandler(uc))
	stream, err := newClient(url).WatchClient(t.Context(),
		connect.NewRequest(&clientsv1.WatchClientRequest{Id: "missing"}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	require.False(t, stream.Receive())
	require.Error(t, stream.Err())
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(stream.Err()))
}

func TestClientsHandler_WatchClient_AlreadyDisconnected(t *testing.T) {
	t.Parallel()

	now := time.Now()
	ctrl := gomock.NewController(t)
	uc := clientsmock.NewMockusecase(ctrl)
	uc.EXPECT().Get(gomock.Any(), "x").Return(
		&domain.Client{ID: "x", DisconnectedAt: &now, ClientName: "svc"},
		nil, nil,
	)

	url := newTestServer(t, clients.NewHandler(uc, clients.WithSnapshotInterval(time.Hour)))
	stream, err := newClient(url).WatchClient(t.Context(),
		connect.NewRequest(&clientsv1.WatchClientRequest{Id: "x"}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	got := receiveFrame(t, stream)
	assert.Equal(t, clientsv1.WatchClientResponse_KIND_DISCONNECTED, got.GetKind())

	// Stream should close right after the single frame — no SubscribeClient call.
	assert.False(t, stream.Receive())
	assert.NoError(t, stream.Err())
}

func TestClientsHandler_WatchClient_InitialSnapshotThenRequestRecorded(t *testing.T) {
	t.Parallel()

	now := time.Now()
	ctrl := gomock.NewController(t)
	uc := clientsmock.NewMockusecase(ctrl)
	uc.EXPECT().Get(gomock.Any(), "x").Return(
		&domain.Client{ID: "x", ClientName: "svc", ConnectedAt: now}, nil, nil,
	).AnyTimes()
	fx := setupSubscribeClient(uc, "x")

	url := newTestServer(t, clients.NewHandler(uc, clients.WithSnapshotInterval(time.Hour)))
	stream, err := newClient(url).WatchClient(t.Context(),
		connect.NewRequest(&clientsv1.WatchClientRequest{Id: "x"}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	snap := receiveFrame(t, stream)
	assert.Equal(t, clientsv1.WatchClientResponse_KIND_SNAPSHOT, snap.GetKind())
	assert.Equal(t, "x", snap.GetClient().GetId())

	fx.push(t, domain.ClientChange{
		Kind:   domain.ClientRequestRecorded,
		Client: &domain.Client{ID: "x"},
		Event:  &domain.ClientEvent{Method: "Put", Key: "/k", Timestamp: now},
	})

	got := receiveFrame(t, stream)
	assert.Equal(t, clientsv1.WatchClientResponse_KIND_REQUEST_RECORDED, got.GetKind())
	require.NotNil(t, got.GetEvent())
	assert.Equal(t, "Put", got.GetEvent().GetMethod())
}

func TestClientsHandler_WatchClient_DisconnectExitsCleanly(t *testing.T) {
	t.Parallel()

	now := time.Now()
	disconn := now.Add(time.Minute)

	ctrl := gomock.NewController(t)
	uc := clientsmock.NewMockusecase(ctrl)
	uc.EXPECT().Get(gomock.Any(), "x").Return(
		&domain.Client{ID: "x", ConnectedAt: now}, nil, nil,
	).AnyTimes()
	fx := setupSubscribeClient(uc, "x")

	url := newTestServer(t, clients.NewHandler(uc, clients.WithSnapshotInterval(time.Hour)))
	stream, err := newClient(url).WatchClient(t.Context(),
		connect.NewRequest(&clientsv1.WatchClientRequest{Id: "x"}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	_ = receiveFrame(t, stream) // initial snapshot

	fx.push(t, domain.ClientChange{
		Kind:   domain.ClientDisconnected,
		Client: &domain.Client{ID: "x", DisconnectedAt: &disconn},
	})

	got := receiveFrame(t, stream)
	assert.Equal(t, clientsv1.WatchClientResponse_KIND_DISCONNECTED, got.GetKind())

	// Stream ends after disconnect; no error.
	assert.False(t, stream.Receive())
	require.NoError(t, stream.Err())
	fx.waitForCancel(t)
}

func TestClientsHandler_WatchClient_CtxCancelReleasesSubscription(t *testing.T) {
	t.Parallel()

	now := time.Now()
	ctrl := gomock.NewController(t)
	uc := clientsmock.NewMockusecase(ctrl)
	uc.EXPECT().Get(gomock.Any(), "x").Return(
		&domain.Client{ID: "x", ConnectedAt: now}, nil, nil,
	).AnyTimes()
	fx := setupSubscribeClient(uc, "x")

	url := newTestServer(t, clients.NewHandler(uc, clients.WithSnapshotInterval(time.Hour)))

	ctx, cancel := context.WithCancel(t.Context())
	stream, err := newClient(url).WatchClient(ctx,
		connect.NewRequest(&clientsv1.WatchClientRequest{Id: "x"}))
	require.NoError(t, err)

	_ = receiveFrame(t, stream)
	cancel()

	fx.waitForCancel(t)
}

func TestClientsHandler_WatchClient_PeriodicSnapshot(t *testing.T) {
	t.Parallel()

	now := time.Now()
	ctrl := gomock.NewController(t)
	uc := clientsmock.NewMockusecase(ctrl)
	uc.EXPECT().Get(gomock.Any(), "x").Return(
		&domain.Client{ID: "x", ConnectedAt: now}, nil, nil,
	).AnyTimes()
	setupSubscribeClient(uc, "x")

	url := newTestServer(t, clients.NewHandler(uc, clients.WithSnapshotInterval(40*time.Millisecond)))
	stream, err := newClient(url).WatchClient(t.Context(),
		connect.NewRequest(&clientsv1.WatchClientRequest{Id: "x"}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	for range 3 {
		got := receiveFrame(t, stream)
		assert.Equal(t, clientsv1.WatchClientResponse_KIND_SNAPSHOT, got.GetKind())
	}
}
