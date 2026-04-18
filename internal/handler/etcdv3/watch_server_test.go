package etcdv3_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"google.golang.org/grpc/metadata"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/etcdv3"
)

// -----------------------------------------------------------------------------
// Fakes: stream, repo, publisher
// -----------------------------------------------------------------------------

// fakeWatchStream implements etcdserverpb.Watch_WatchServer for tests.
//
//nolint:containedctx // ctx is stored to implement the Watch_WatchServer.Context() interface method
type fakeWatchStream struct {
	ctx context.Context

	mu        sync.Mutex
	reqQueue  []*etcdserverpb.WatchRequest
	reqWaiter chan struct{}
	closed    bool

	respMu sync.Mutex
	resps  []*etcdserverpb.WatchResponse
}

func newFakeStream(ctx context.Context) *fakeWatchStream {
	return &fakeWatchStream{
		ctx:       ctx,
		reqWaiter: make(chan struct{}, 256),
	}
}

func (s *fakeWatchStream) Context() context.Context { return s.ctx }

func (s *fakeWatchStream) Send(resp *etcdserverpb.WatchResponse) error {
	s.respMu.Lock()
	defer s.respMu.Unlock()

	s.resps = append(s.resps, resp)

	return nil
}

func (s *fakeWatchStream) Recv() (*etcdserverpb.WatchRequest, error) {
	for {
		s.mu.Lock()

		if s.closed && len(s.reqQueue) == 0 {
			s.mu.Unlock()

			return nil, io.EOF
		}

		if len(s.reqQueue) > 0 {
			req := s.reqQueue[0]
			s.reqQueue = s.reqQueue[1:]
			s.mu.Unlock()

			return req, nil
		}

		s.mu.Unlock()

		select {
		case <-s.reqWaiter:
			// loop to re-check queue
		case <-s.ctx.Done():
			return nil, fmt.Errorf("recv: %w", s.ctx.Err())
		}
	}
}

// ServerStream no-op methods — unused in tests.
func (s *fakeWatchStream) SetHeader(metadata.MD) error  { return nil }
func (s *fakeWatchStream) SendHeader(metadata.MD) error { return nil }
func (s *fakeWatchStream) SetTrailer(metadata.MD)       {}
func (s *fakeWatchStream) SendMsg(any) error            { return nil }
func (s *fakeWatchStream) RecvMsg(any) error            { return nil }

// pushReq queues a request for Watch() to consume.
func (s *fakeWatchStream) pushReq(req *etcdserverpb.WatchRequest) {
	s.mu.Lock()
	s.reqQueue = append(s.reqQueue, req)
	s.mu.Unlock()

	select {
	case s.reqWaiter <- struct{}{}:
	default:
	}
}

// close signals that no more requests will come — Watch() should exit with io.EOF.
func (s *fakeWatchStream) close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()

	select {
	case s.reqWaiter <- struct{}{}:
	default:
	}
}

func (s *fakeWatchStream) snapshotResps() []*etcdserverpb.WatchResponse {
	s.respMu.Lock()
	defer s.respMu.Unlock()

	out := make([]*etcdserverpb.WatchResponse, len(s.resps))
	copy(out, s.resps)

	return out
}

// fakeWatchRepo implements WatchRepo backed by in-memory data.
type fakeWatchRepo struct {
	rev          int64
	entries      []*domain.ChangelogEntry
	kvAtRevision map[string][]byte // key: "rev/ns/path"
	listErr      error
}

func (r *fakeWatchRepo) CurrentRevisionValue(_ context.Context) (int64, error) {
	return r.rev, nil
}

func (r *fakeWatchRepo) ListChanges(_ context.Context, since int64, limit int) ([]*domain.ChangelogEntry, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}

	var out []*domain.ChangelogEntry
	for _, e := range r.entries {
		if e.Revision <= since {
			continue
		}

		out = append(out, e)

		if len(out) >= limit {
			break
		}
	}

	return out, nil
}

func (r *fakeWatchRepo) GetKVAtRevision(_ context.Context, ns, path string, rev int64) ([]byte, error) {
	return r.kvAtRevision[kvKey(rev, ns, path)], nil
}

func kvKey(rev int64, ns, path string) string {
	return ns + "\x00" + path + "\x00" + string(rune(rev))
}

// fakeWatchPublisher is a controllable publisher for tests.
type fakeWatchPublisher struct {
	mu            sync.Mutex
	subscriptions []*fakeSub
}

type fakeSub struct {
	prefix    string
	namespace string
	ch        chan domain.WatchEvent
	canceled  bool
}

func (p *fakeWatchPublisher) Subscribe(_ context.Context, prefix, ns string) (<-chan domain.WatchEvent, func()) {
	p.mu.Lock()
	defer p.mu.Unlock()

	s := &fakeSub{prefix: prefix, namespace: ns, ch: make(chan domain.WatchEvent, 64)}
	p.subscriptions = append(p.subscriptions, s)

	return s.ch, func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		if !s.canceled {
			s.canceled = true
			close(s.ch)
		}
	}
}

// push sends an event to all active subscribers (tests assume single subscriber).
func (p *fakeWatchPublisher) push(ev domain.WatchEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, s := range p.subscriptions {
		if s.canceled {
			continue
		}

		select {
		case s.ch <- ev:
		default:
		}
	}
}

func (p *fakeWatchPublisher) subscriptionCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	active := 0
	for _, s := range p.subscriptions {
		if !s.canceled {
			active++
		}
	}

	return active
}

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

// waitForResps polls until the stream has at least n responses or times out.
func waitForResps(t *testing.T, s *fakeWatchStream, n int) []*etcdserverpb.WatchResponse {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		r := s.snapshotResps()
		if len(r) >= n {
			return r
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %d responses (got %d)", n, len(s.snapshotResps()))

	return nil
}

func TestWatchServer_Create_SendsCreatedAck(t *testing.T) {
	t.Parallel()

	repo := &fakeWatchRepo{rev: 3}
	pub := &fakeWatchPublisher{}
	ws := etcdv3.NewWatchServer(repo, pub)

	ctx, cancel := context.WithCancel(context.Background())
	stream := newFakeStream(ctx)

	done := make(chan error, 1)
	go func() { done <- ws.Watch(stream) }()

	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CreateRequest{
			CreateRequest: &etcdserverpb.WatchCreateRequest{Key: []byte("/default/foo")},
		},
	})

	resps := waitForResps(t, stream, 1)
	assert.True(t, resps[0].Created, "first response must be Created ack")
	assert.Equal(t, int64(1), resps[0].WatchId, "first watch id is 1")
	assert.Equal(t, int64(3), resps[0].Header.Revision)

	cancel()
	<-done
}

func TestWatchServer_RealtimeEvent_Delivered(t *testing.T) {
	t.Parallel()

	repo := &fakeWatchRepo{rev: 0}
	pub := &fakeWatchPublisher{}
	ws := etcdv3.NewWatchServer(repo, pub)

	ctx, cancel := context.WithCancel(context.Background())
	stream := newFakeStream(ctx)

	done := make(chan error, 1)
	go func() { done <- ws.Watch(stream) }()

	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CreateRequest{
			CreateRequest: &etcdserverpb.WatchCreateRequest{
				Key:      []byte("/default/"),
				RangeEnd: []byte("/default0"),
			},
		},
	})

	// Wait for Created ack before firing an event
	waitForResps(t, stream, 1)

	// Fire event
	pub.push(domain.WatchEvent{
		Type:      domain.EventTypeCreated,
		Path:      "/foo",
		Namespace: "default",
		Revision:  5,
		Config: &domain.Config{
			Path: "/foo", Namespace: "default",
			Content: "v1", Revision: 5, CreateRevision: 5, Version: 1,
		},
	})

	resps := waitForResps(t, stream, 2)
	ev := resps[1]
	require.Len(t, ev.Events, 1)
	assert.Equal(t, mvccpb.PUT, ev.Events[0].Type)
	assert.Equal(t, []byte("/default/foo"), ev.Events[0].Kv.Key)
	assert.Equal(t, int64(5), ev.Events[0].Kv.ModRevision)
	assert.Equal(t, int64(5), ev.Header.Revision, "header carries event revision")

	cancel()
	<-done
}

func TestWatchServer_FiltersByNamespace(t *testing.T) {
	t.Parallel()

	repo := &fakeWatchRepo{rev: 0}
	pub := &fakeWatchPublisher{}
	ws := etcdv3.NewWatchServer(repo, pub)

	ctx, cancel := context.WithCancel(context.Background())
	stream := newFakeStream(ctx)

	done := make(chan error, 1)
	go func() { done <- ws.Watch(stream) }()

	// Single-key watch
	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CreateRequest{
			CreateRequest: &etcdserverpb.WatchCreateRequest{Key: []byte("/default/foo")},
		},
	})

	waitForResps(t, stream, 1)

	// Miss: wrong namespace — should NOT produce an event.
	// (Publisher's own filter will drop it since we subscribe on "default";
	// this tests the subscription filter path.)
	pub.push(domain.WatchEvent{Path: "/foo", Namespace: "other", Revision: 2})

	// Hit
	pub.push(domain.WatchEvent{
		Path: "/foo", Namespace: "default", Revision: 3,
		Config: &domain.Config{Path: "/foo", Namespace: "default", Revision: 3, Version: 1},
	})

	resps := waitForResps(t, stream, 2)
	assert.Equal(t, []byte("/default/foo"), resps[1].Events[0].Kv.Key)

	cancel()
	<-done
}

func TestWatchServer_FiltersByExactPathSingleKey(t *testing.T) {
	t.Parallel()

	// Single-key watch should NOT fire on sibling keys.
	repo := &fakeWatchRepo{rev: 0}
	pub := &fakeWatchPublisher{}
	ws := etcdv3.NewWatchServer(repo, pub)

	ctx, cancel := context.WithCancel(context.Background())
	stream := newFakeStream(ctx)

	done := make(chan error, 1)
	go func() { done <- ws.Watch(stream) }()

	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CreateRequest{
			CreateRequest: &etcdserverpb.WatchCreateRequest{Key: []byte("/default/foo")},
		},
	})
	waitForResps(t, stream, 1)

	// Push a PUT for /foo.json (prefix-matches "/foo" but NOT exact).
	// Publisher subscription is on prefix="/foo" — it will forward this event
	// to runWatcher, which must drop it via matchesEvent's exact-match check.
	pub.push(domain.WatchEvent{
		Path: "/foo.json", Namespace: "default", Revision: 2,
		Config: &domain.Config{Path: "/foo.json", Namespace: "default", Revision: 2, Version: 1},
	})

	// Now push exact match
	pub.push(domain.WatchEvent{
		Path: "/foo", Namespace: "default", Revision: 3,
		Config: &domain.Config{Path: "/foo", Namespace: "default", Revision: 3, Version: 1},
	})

	resps := waitForResps(t, stream, 2)
	require.Len(t, resps, 2, "should only receive ack + exact match")
	assert.Equal(t, []byte("/default/foo"), resps[1].Events[0].Kv.Key)

	cancel()
	<-done
}

func TestWatchServer_HistoricalReplay(t *testing.T) {
	t.Parallel()

	repo := &fakeWatchRepo{
		rev: 5,
		entries: []*domain.ChangelogEntry{
			{Revision: 2, Type: domain.EventTypeCreated, Path: "/foo", Namespace: "default", Version: 1},
			{Revision: 4, Type: domain.EventTypeUpdated, Path: "/foo", Namespace: "default", Version: 2},
		},
		kvAtRevision: map[string][]byte{
			kvKey(2, "default", "/foo"): []byte("v1"),
			kvKey(4, "default", "/foo"): []byte("v2"),
		},
	}
	pub := &fakeWatchPublisher{}
	ws := etcdv3.NewWatchServer(repo, pub)

	ctx, cancel := context.WithCancel(context.Background())
	stream := newFakeStream(ctx)

	done := make(chan error, 1)
	go func() { done <- ws.Watch(stream) }()

	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CreateRequest{
			CreateRequest: &etcdserverpb.WatchCreateRequest{
				Key: []byte("/default/foo"), StartRevision: 1,
			},
		},
	})

	resps := waitForResps(t, stream, 3) // Created + 2 history entries

	assert.True(t, resps[0].Created)
	assert.Equal(t, int64(2), resps[1].Events[0].Kv.ModRevision)
	assert.Equal(t, []byte("v1"), resps[1].Events[0].Kv.Value)
	assert.Equal(t, int64(4), resps[2].Events[0].Kv.ModRevision)
	assert.Equal(t, []byte("v2"), resps[2].Events[0].Kv.Value)

	cancel()
	<-done
}

func TestWatchServer_HistoricalReplay_DeletesPublished(t *testing.T) {
	t.Parallel()

	repo := &fakeWatchRepo{
		rev: 3,
		entries: []*domain.ChangelogEntry{
			{Revision: 2, Type: domain.EventTypeDeleted, Path: "/foo", Namespace: "default"},
		},
	}
	ws := etcdv3.NewWatchServer(repo, &fakeWatchPublisher{})

	ctx, cancel := context.WithCancel(context.Background())
	stream := newFakeStream(ctx)

	done := make(chan error, 1)
	go func() { done <- ws.Watch(stream) }()

	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CreateRequest{
			CreateRequest: &etcdserverpb.WatchCreateRequest{Key: []byte("/default/foo"), StartRevision: 1},
		},
	})

	resps := waitForResps(t, stream, 2)
	assert.True(t, resps[0].Created)
	assert.Equal(t, mvccpb.DELETE, resps[1].Events[0].Type)
	assert.Equal(t, int64(2), resps[1].Events[0].Kv.ModRevision)
	assert.Equal(t, int64(0), resps[1].Events[0].Kv.Version, "delete version=0")

	cancel()
	<-done
}

func TestWatchServer_HistoricalDedupesWithRealtime(t *testing.T) {
	t.Parallel()

	// Verify dedup: a realtime event for a revision already sent via replay must be dropped.
	repo := &fakeWatchRepo{
		rev: 5,
		entries: []*domain.ChangelogEntry{
			{Revision: 3, Type: domain.EventTypeCreated, Path: "/foo", Namespace: "default", Version: 1},
		},
		kvAtRevision: map[string][]byte{
			kvKey(3, "default", "/foo"): []byte("v1"),
		},
	}
	pub := &fakeWatchPublisher{}
	ws := etcdv3.NewWatchServer(repo, pub)

	ctx, cancel := context.WithCancel(context.Background())
	stream := newFakeStream(ctx)

	done := make(chan error, 1)
	go func() { done <- ws.Watch(stream) }()

	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CreateRequest{
			CreateRequest: &etcdserverpb.WatchCreateRequest{
				Key: []byte("/default/foo"), StartRevision: 1,
			},
		},
	})

	// Wait for Created + historical event
	waitForResps(t, stream, 2)

	// Push a realtime event with rev=3 (duplicate) and rev=7 (new).
	pub.push(domain.WatchEvent{
		Path: "/foo", Namespace: "default", Revision: 3,
		Config: &domain.Config{Path: "/foo", Namespace: "default", Revision: 3, Version: 1},
	})
	pub.push(domain.WatchEvent{
		Path: "/foo", Namespace: "default", Revision: 7,
		Config: &domain.Config{Path: "/foo", Namespace: "default", Revision: 7, Version: 2},
	})

	resps := waitForResps(t, stream, 3)
	require.Len(t, resps, 3, "duplicate rev=3 must be dropped, only rev=7 delivered")
	assert.Equal(t, int64(7), resps[2].Events[0].Kv.ModRevision)

	cancel()
	<-done
}

func TestWatchServer_Cancel_StopsWatch(t *testing.T) {
	t.Parallel()

	repo := &fakeWatchRepo{rev: 0}
	pub := &fakeWatchPublisher{}
	ws := etcdv3.NewWatchServer(repo, pub)

	ctx := t.Context()

	stream := newFakeStream(ctx)
	done := make(chan error, 1)
	go func() { done <- ws.Watch(stream) }()

	// Create
	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CreateRequest{
			CreateRequest: &etcdserverpb.WatchCreateRequest{Key: []byte("/default/foo")},
		},
	})
	waitForResps(t, stream, 1)
	assert.Equal(t, 1, pub.subscriptionCount(), "subscribed after create")

	// Cancel
	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CancelRequest{
			CancelRequest: &etcdserverpb.WatchCancelRequest{WatchId: 1},
		},
	})
	resps := waitForResps(t, stream, 2)
	assert.True(t, resps[1].Canceled, "cancel ack sent")
	assert.Equal(t, int64(1), resps[1].WatchId)

	// Allow goroutine to unsubscribe
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if pub.subscriptionCount() == 0 {
			break
		}

		time.Sleep(5 * time.Millisecond)
	}
	assert.Equal(t, 0, pub.subscriptionCount(), "subscription released after cancel")

	stream.close()
	<-done
}

func TestWatchServer_Progress(t *testing.T) {
	t.Parallel()

	repo := &fakeWatchRepo{rev: 9}
	ws := etcdv3.NewWatchServer(repo, &fakeWatchPublisher{})

	ctx, cancel := context.WithCancel(context.Background())
	stream := newFakeStream(ctx)

	done := make(chan error, 1)
	go func() { done <- ws.Watch(stream) }()

	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_ProgressRequest{
			ProgressRequest: &etcdserverpb.WatchProgressRequest{},
		},
	})

	resps := waitForResps(t, stream, 1)
	assert.Equal(t, int64(-1), resps[0].WatchId, "progress response uses WatchId=-1")
	assert.Equal(t, int64(9), resps[0].Header.Revision)

	cancel()
	<-done
}

func TestWatchServer_StreamEOF_CleansUpSubscriptions(t *testing.T) {
	t.Parallel()

	repo := &fakeWatchRepo{rev: 0}
	pub := &fakeWatchPublisher{}
	ws := etcdv3.NewWatchServer(repo, pub)

	ctx := t.Context()

	stream := newFakeStream(ctx)
	done := make(chan error, 1)
	go func() { done <- ws.Watch(stream) }()

	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CreateRequest{
			CreateRequest: &etcdserverpb.WatchCreateRequest{Key: []byte("/default/foo")},
		},
	})
	waitForResps(t, stream, 1)
	assert.Equal(t, 1, pub.subscriptionCount())

	stream.close()

	err := <-done
	require.NoError(t, err, "EOF must return nil, not an error")

	// After Watch returns, subscriptions should be released.
	assert.Equal(t, 0, pub.subscriptionCount(), "subscriptions cleaned up on stream close")
}

func TestWatchServer_InvalidKey_ReturnsError(t *testing.T) {
	t.Parallel()

	repo := &fakeWatchRepo{rev: 0}
	ws := etcdv3.NewWatchServer(repo, &fakeWatchPublisher{})

	ctx := t.Context()

	stream := newFakeStream(ctx)
	done := make(chan error, 1)
	go func() { done <- ws.Watch(stream) }()

	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CreateRequest{
			CreateRequest: &etcdserverpb.WatchCreateRequest{Key: []byte("bad")},
		},
	})

	err := <-done
	require.Error(t, err, "invalid key returns from Watch() with an error")
}

func TestWatchServer_MultipleWatchers_DistinctIDs(t *testing.T) {
	t.Parallel()

	repo := &fakeWatchRepo{rev: 0}
	pub := &fakeWatchPublisher{}
	ws := etcdv3.NewWatchServer(repo, pub)

	ctx := t.Context()

	stream := newFakeStream(ctx)
	done := make(chan error, 1)
	go func() { done <- ws.Watch(stream) }()

	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CreateRequest{
			CreateRequest: &etcdserverpb.WatchCreateRequest{Key: []byte("/a/x")},
		},
	})
	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CreateRequest{
			CreateRequest: &etcdserverpb.WatchCreateRequest{Key: []byte("/b/y")},
		},
	})

	resps := waitForResps(t, stream, 2)
	require.True(t, resps[0].Created)
	require.True(t, resps[1].Created)
	assert.Equal(t, int64(1), resps[0].WatchId)
	assert.Equal(t, int64(2), resps[1].WatchId, "IDs are monotonic per stream")

	stream.close()
	<-done
}

// fakeWatchTracker counts Register/Unregister per connection ID and remembers
// the most recent registration payload for assertions.
type fakeWatchTracker struct {
	mu   sync.Mutex
	n    map[string]int
	last map[string]domain.ActiveWatch
}

func newFakeWatchTracker() *fakeWatchTracker {
	return &fakeWatchTracker{n: map[string]int{}}
}

func (t *fakeWatchTracker) RegisterWatch(connID string, w domain.ActiveWatch) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.n[connID]++

	if t.last == nil {
		t.last = make(map[string]domain.ActiveWatch)
	}
	t.last[connID] = w
}

func (t *fakeWatchTracker) UnregisterWatch(connID string, _ int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.n[connID]--
}

func (t *fakeWatchTracker) get(connID string) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.n[connID]
}

type connIDKeyType struct{}

//nolint:gochecknoglobals // context key — standard Go pattern
var connIDKeyForTest connIDKeyType

func TestWatchServer_Tracker_IncOnCreateDecOnCancel(t *testing.T) {
	t.Parallel()

	repo := &fakeWatchRepo{rev: 0}
	pub := &fakeWatchPublisher{}
	tr := newFakeWatchTracker()
	ws := etcdv3.NewWatchServer(repo, pub).WithTracker(tr, func(ctx context.Context) string {
		v, _ := ctx.Value(connIDKeyForTest).(string)

		return v
	})

	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), connIDKeyForTest, "conn-A"))
	defer cancel()
	stream := newFakeStream(ctx)

	done := make(chan error, 1)
	go func() { done <- ws.Watch(stream) }()

	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CreateRequest{
			CreateRequest: &etcdserverpb.WatchCreateRequest{Key: []byte("/default/foo")},
		},
	})
	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CreateRequest{
			CreateRequest: &etcdserverpb.WatchCreateRequest{Key: []byte("/default/bar")},
		},
	})

	// Two watches → +2.
	waitForResps(t, stream, 2)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if tr.get("conn-A") == 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	assert.Equal(t, 2, tr.get("conn-A"), "Inc fires per create")

	// Cancel one → -1.
	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CancelRequest{
			CancelRequest: &etcdserverpb.WatchCancelRequest{WatchId: 1},
		},
	})
	waitForResps(t, stream, 3)
	for time.Now().Before(deadline) {
		if tr.get("conn-A") == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	assert.Equal(t, 1, tr.get("conn-A"), "cancel decrements")

	// Stream closes → final defer releases the remaining watcher.
	stream.close()
	<-done

	for time.Now().Before(deadline) {
		if tr.get("conn-A") == 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	assert.Equal(t, 0, tr.get("conn-A"), "stream EOF must dec the remaining watcher")
}

func TestWatchServer_Tracker_PassesWatchDetail(t *testing.T) {
	t.Parallel()

	repo := &fakeWatchRepo{rev: 0}
	pub := &fakeWatchPublisher{}
	tr := newFakeWatchTracker()
	ws := etcdv3.NewWatchServer(repo, pub).WithTracker(tr, func(ctx context.Context) string {
		v, _ := ctx.Value(connIDKeyForTest).(string)

		return v
	})

	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), connIDKeyForTest, "conn-X"))
	defer cancel()
	stream := newFakeStream(ctx)

	done := make(chan error, 1)
	go func() { done <- ws.Watch(stream) }()

	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CreateRequest{
			CreateRequest: &etcdserverpb.WatchCreateRequest{
				Key:            []byte("/default/foo"),
				RangeEnd:       []byte("/default/bar"),
				StartRevision:  42,
				PrevKv:         true,
				ProgressNotify: true,
			},
		},
	})

	waitForResps(t, stream, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if tr.get("conn-X") == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	tr.mu.Lock()
	got := tr.last["conn-X"]
	tr.mu.Unlock()

	assert.Equal(t, "/default/foo", got.StartKey)
	assert.Equal(t, "/default/bar", got.EndKey)
	assert.Equal(t, int64(42), got.StartRevision)
	assert.True(t, got.PrevKv)
	assert.True(t, got.ProgressNotify)
	assert.False(t, got.CreatedAt.IsZero())
	assert.Equal(t, int64(1), got.WatchID, "first watch in stream → ID 1")

	stream.close()
	<-done
}

func TestWatchServer_Tracker_NoConnID_NoTracking(t *testing.T) {
	t.Parallel()

	repo := &fakeWatchRepo{rev: 0}
	pub := &fakeWatchPublisher{}
	tr := newFakeWatchTracker()
	// extractor returns "" → no tracking
	ws := etcdv3.NewWatchServer(repo, pub).WithTracker(tr, func(context.Context) string { return "" })

	ctx := t.Context()
	stream := newFakeStream(ctx)

	done := make(chan error, 1)
	go func() { done <- ws.Watch(stream) }()

	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CreateRequest{
			CreateRequest: &etcdserverpb.WatchCreateRequest{Key: []byte("/default/foo")},
		},
	})
	waitForResps(t, stream, 1)

	stream.close()
	<-done

	assert.Equal(t, 0, tr.get(""), "no connID → no Inc/Dec calls")
}

func TestWatchServer_ReplayError_IsTolerated(t *testing.T) {
	t.Parallel()

	repo := &fakeWatchRepo{rev: 5, listErr: errors.New("db unavailable")}
	ws := etcdv3.NewWatchServer(repo, &fakeWatchPublisher{})

	ctx := t.Context()

	stream := newFakeStream(ctx)
	done := make(chan error, 1)
	go func() { done <- ws.Watch(stream) }()

	stream.pushReq(&etcdserverpb.WatchRequest{
		RequestUnion: &etcdserverpb.WatchRequest_CreateRequest{
			CreateRequest: &etcdserverpb.WatchCreateRequest{
				Key: []byte("/default/foo"), StartRevision: 1,
			},
		},
	})

	// Even though replay errors, Watch() must keep running — replay failure is internal.
	waitForResps(t, stream, 1) // Created ack delivered before replay starts

	// Close stream to exit Watch cleanly.
	stream.close()
	<-done
}
