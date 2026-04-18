package etcdv3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// WatchRepo is the storage surface the Watch server needs.
type WatchRepo interface {
	CurrentRevisionValue(ctx context.Context) (int64, error)
	ListChanges(ctx context.Context, sinceRevision int64, limit int) ([]*domain.ChangelogEntry, error)
	GetKVAtRevision(ctx context.Context, namespace, path string, revision int64) ([]byte, error)
}

// WatchPublisher is the pub/sub surface for realtime events.
type WatchPublisher interface {
	Subscribe(ctx context.Context, pathPrefix, namespace string) (<-chan domain.WatchEvent, func())
}

// WatchTracker is the optional surface for the monitor registry. WatchServer
// uses it to maintain per-connection active-watch detail. May be nil.
type WatchTracker interface {
	RegisterWatch(connID string, w domain.ActiveWatch)
	UnregisterWatch(connID string, watchID int64)
}

// ConnIDExtractor pulls the per-connection ID out of a stream context.
// Decoupled from the transport package so etcdv3 has no transport dependency.
type ConnIDExtractor func(ctx context.Context) string

type WatchServer struct {
	etcdserverpb.UnimplementedWatchServer

	repo      WatchRepo
	publisher WatchPublisher
	tracker   WatchTracker
	connID    ConnIDExtractor
}

func NewWatchServer(repo WatchRepo, publisher WatchPublisher) *WatchServer {
	return &WatchServer{repo: repo, publisher: publisher}
}

// WithTracker enables per-connection active-watch tracking via the supplied
// tracker. extractor returns the connection ID for a stream context; if nil,
// tracking is silently disabled.
func (s *WatchServer) WithTracker(tracker WatchTracker, extractor ConnIDExtractor) *WatchServer {
	s.tracker = tracker
	s.connID = extractor

	return s
}

type watcher struct {
	id          int64
	start, end  []byte
	namespace   string
	pathPrefix  string
	scanAll     bool
	singleKey   bool
	exactPath   string // set when singleKey — the exact config path to match
	prevKv      bool
	progress    bool
	cancel      context.CancelFunc
	unsubscribe func()

	// tracked carries the connID this watcher was registered against, or "" if
	// not tracked. release() is idempotent against this state — cleared after
	// the first release so a double-release doesn't double-unregister.
	tracked string
}

func (s *WatchServer) Watch(stream etcdserverpb.Watch_WatchServer) error {
	ctx := stream.Context()

	var (
		mu       sync.Mutex
		watchers = make(map[int64]*watcher)
		nextID   atomic.Int64
	)

	// Single send-lock — grpc streams are not safe for concurrent Send.
	var sendMu sync.Mutex

	send := func(resp *etcdserverpb.WatchResponse) error {
		sendMu.Lock()
		defer sendMu.Unlock()

		return stream.Send(resp)
	}

	defer func() {
		mu.Lock()
		toRelease := make([]*watcher, 0, len(watchers))

		for _, w := range watchers {
			toRelease = append(toRelease, w)
		}

		watchers = nil // make any further mutations a no-op
		mu.Unlock()

		s.releaseWatchers(toRelease)
	}()

	for {
		req, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				return nil
			}

			return fmt.Errorf("recv watch request: %w", err)
		}

		if err := s.handleWatchRequest(ctx, req, &mu, watchers, &nextID, send); err != nil {
			return err
		}
	}
}

func (s *WatchServer) handleWatchRequest(
	ctx context.Context,
	req *etcdserverpb.WatchRequest,
	mu *sync.Mutex,
	watchers map[int64]*watcher,
	nextID *atomic.Int64,
	send func(*etcdserverpb.WatchResponse) error,
) error {
	switch r := req.RequestUnion.(type) {
	case *etcdserverpb.WatchRequest_CreateRequest:
		w, err := s.createWatcher(ctx, r.CreateRequest, nextID.Add(1), send)
		if err != nil {
			return err
		}

		mu.Lock()
		watchers[w.id] = w
		mu.Unlock()

	case *etcdserverpb.WatchRequest_CancelRequest:
		id := r.CancelRequest.WatchId

		mu.Lock()
		w, ok := watchers[id]
		delete(watchers, id)
		mu.Unlock()

		if ok {
			s.releaseWatcher(w)
		}

		rev, _ := s.repo.CurrentRevisionValue(ctx)

		if err := send(&etcdserverpb.WatchResponse{
			Header:   newHeader(rev),
			WatchId:  id,
			Canceled: true,
		}); err != nil {
			return err
		}

	case *etcdserverpb.WatchRequest_ProgressRequest:
		rev, _ := s.repo.CurrentRevisionValue(ctx)

		if err := send(&etcdserverpb.WatchResponse{
			Header:  newHeader(rev),
			WatchId: -1,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *WatchServer) releaseWatchers(ws []*watcher) {
	for _, w := range ws {
		s.releaseWatcher(w)
	}
}

func (s *WatchServer) createWatcher(
	ctx context.Context,
	req *etcdserverpb.WatchCreateRequest,
	id int64,
	send func(*etcdserverpb.WatchResponse) error,
) (*watcher, error) {
	startNS, startPath, endNS, endPath, ok := SplitRange(req.Key, req.RangeEnd)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "invalid watch key: %q", string(req.Key))
	}

	scanAll := endNS == "\x00"
	singleKey := endNS == "" && endPath == ""

	// Derive the publisher filter. Publisher filters cheaply by namespace + pathPrefix;
	// the fine-grained single-key / cross-namespace filtering happens in matchesEvent.
	var (
		subNamespace string
		subPrefix    string
	)

	switch {
	case scanAll:
		// Leave both empty — subscribe to every event.

	case singleKey:
		subNamespace = startNS
		subPrefix = startPath

	case startNS == endNS:
		// Bounded within one namespace — subscribe to that namespace with a common prefix.
		subNamespace = startNS
		subPrefix = commonPathPrefix(startPath, endPath)

	default:
		// Cross-namespace range — must receive events from all namespaces and filter in matchesEvent.
		subNamespace = ""
		subPrefix = ""
	}

	currentRev, err := s.repo.CurrentRevisionValue(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get revision: %v", err)
	}

	// Ack watcher creation.
	if err := send(&etcdserverpb.WatchResponse{
		Header:  newHeader(currentRev),
		WatchId: id,
		Created: true,
	}); err != nil {
		return nil, err
	}

	// Subscribe to realtime events *before* replaying history so we don't miss events.
	subCtx, cancel := context.WithCancel(ctx)
	events, unsubscribe := s.publisher.Subscribe(subCtx, subPrefix, subNamespace)

	w := &watcher{
		id:          id,
		start:       req.Key,
		end:         req.RangeEnd,
		namespace:   subNamespace,
		pathPrefix:  subPrefix,
		scanAll:     scanAll,
		singleKey:   singleKey,
		prevKv:      req.PrevKv,
		progress:    req.ProgressNotify,
		cancel:      cancel,
		unsubscribe: unsubscribe,
	}

	if singleKey {
		w.exactPath = startPath
		w.namespace = startNS
	}

	s.trackWatcher(ctx, w, req)

	go s.runWatcher(subCtx, w, req.StartRevision, currentRev, events, send)

	return w, nil
}

// trackWatcher registers an active-watch against the originating connection,
// if a tracker and connection ID extractor are configured.
func (s *WatchServer) trackWatcher(ctx context.Context, w *watcher, req *etcdserverpb.WatchCreateRequest) {
	if s.tracker == nil || s.connID == nil {
		return
	}

	cid := s.connID(ctx)
	if cid == "" {
		return
	}

	s.tracker.RegisterWatch(cid, domain.ActiveWatch{
		WatchID:        w.id,
		StartKey:       string(req.Key),
		EndKey:         string(req.RangeEnd),
		StartRevision:  req.StartRevision,
		CreatedAt:      time.Now(),
		PrevKv:         req.PrevKv,
		ProgressNotify: req.ProgressNotify,
	})
	w.tracked = cid
}

// releaseWatcher is the single point of teardown for a watcher. Idempotent
// against repeated calls: each Register must be matched by exactly one
// Unregister, so w.tracked is cleared after the first release.
func (s *WatchServer) releaseWatcher(w *watcher) {
	if w == nil {
		return
	}

	if w.unsubscribe != nil {
		w.unsubscribe()
	}

	if w.cancel != nil {
		w.cancel()
	}

	if w.tracked != "" && s.tracker != nil {
		s.tracker.UnregisterWatch(w.tracked, w.id)
		w.tracked = "" // ensure no double-unregister
	}
}

func (s *WatchServer) runWatcher(
	ctx context.Context,
	w *watcher,
	startRevision, currentRev int64,
	events <-chan domain.WatchEvent,
	send func(*etcdserverpb.WatchResponse) error,
) {
	var lastSent int64

	// Historical catch-up. We subscribed *before* replay to avoid gaps; as a result,
	// realtime events for revisions already covered by replay may arrive duplicated
	// via `events`. Track lastSent to dedupe.
	if startRevision > 0 && startRevision <= currentRev {
		lastRev, err := s.replayHistory(ctx, w, startRevision, send)
		if err != nil {
			slog.Warn("watch replay failed", "error", err, "watch_id", w.id)

			return
		}

		lastSent = lastRev
	}

	// Realtime loop.
	for {
		select {
		case <-ctx.Done():
			return

		case ev, ok := <-events:
			if !ok {
				return
			}

			newLastSent, err := s.forwardEvent(w, ev, lastSent, send)
			if err != nil {
				slog.Debug("watch send failed", "error", err, "watch_id", w.id)

				return
			}

			lastSent = newLastSent
		}
	}
}

// forwardEvent applies match + dedup filtering and sends the event to the
// client. Returns the updated lastSent revision.
func (s *WatchServer) forwardEvent(
	w *watcher,
	ev domain.WatchEvent,
	lastSent int64,
	send func(*etcdserverpb.WatchResponse) error,
) (int64, error) {
	if !w.matchesEvent(ev) {
		return lastSent, nil
	}

	rev := revisionOfEvent(ev)
	if rev > 0 && rev <= lastSent {
		return lastSent, nil
	}

	resp := &etcdserverpb.WatchResponse{
		Header:  newHeader(rev),
		WatchId: w.id,
		Events:  []*mvccpb.Event{eventToProto(ev)},
	}

	if err := send(resp); err != nil {
		return lastSent, err
	}

	if rev > lastSent {
		lastSent = rev
	}

	return lastSent, nil
}

func (s *WatchServer) replayHistory(
	ctx context.Context,
	w *watcher,
	startRevision int64,
	send func(*etcdserverpb.WatchResponse) error,
) (int64, error) {
	const batchSize = 256

	since := startRevision - 1
	var lastSent int64

	for {
		if err := ctx.Err(); err != nil {
			return lastSent, fmt.Errorf("replay history: %w", err)
		}

		entries, err := s.repo.ListChanges(ctx, since, batchSize)
		if err != nil {
			return lastSent, fmt.Errorf("list changes for replay: %w", err)
		}

		if len(entries) == 0 {
			return lastSent, nil
		}

		for _, e := range entries {
			if err := ctx.Err(); err != nil {
				return lastSent, fmt.Errorf("replay entry: %w", err)
			}

			since = e.Revision

			newLastSent, err := s.replayEntry(ctx, w, e, lastSent, send)
			if err != nil {
				return lastSent, err
			}

			lastSent = newLastSent
		}

		if len(entries) < batchSize {
			return lastSent, nil
		}
	}
}

// replayEntry sends a single changelog entry to the watcher's stream if it
// matches the watcher's key range. Returns the (possibly updated) lastSent
// revision.
func (s *WatchServer) replayEntry(
	ctx context.Context,
	w *watcher,
	e *domain.ChangelogEntry,
	lastSent int64,
	send func(*etcdserverpb.WatchResponse) error,
) (int64, error) {
	if !w.matchesChangelog(e) {
		return lastSent, nil
	}

	var value []byte

	if e.Type != domain.EventTypeDeleted {
		v, err := s.repo.GetKVAtRevision(ctx, e.Namespace, e.Path, e.Revision)
		if err != nil {
			slog.Warn("watch history lookup failed",
				"error", err,
				"ns", e.Namespace,
				"path", e.Path,
				"rev", e.Revision)
		}

		value = v
	}

	resp := &etcdserverpb.WatchResponse{
		Header:  newHeader(e.Revision),
		WatchId: w.id,
		Events:  []*mvccpb.Event{changelogToEvent(e, value)},
	}

	if err := send(resp); err != nil {
		return lastSent, err
	}

	if e.Revision > lastSent {
		lastSent = e.Revision
	}

	return lastSent, nil
}

func (w *watcher) matchesEvent(ev domain.WatchEvent) bool {
	return w.matchesKey(ev.Namespace, ev.Path)
}

func (w *watcher) matchesChangelog(e *domain.ChangelogEntry) bool {
	return w.matchesKey(e.Namespace, e.Path)
}

// matchesKey implements etcd range semantics over our (namespace, path) encoding.
// Keys outside the originally requested [key, range_end) range are rejected.
func (w *watcher) matchesKey(namespace, path string) bool {
	if w.scanAll {
		return true
	}

	if w.singleKey {
		return namespace == w.namespace && path == w.exactPath
	}

	encoded := JoinKey(namespace, path)
	if bytes.Compare(encoded, w.start) < 0 {
		return false
	}

	if len(w.end) > 0 && bytes.Compare(encoded, w.end) >= 0 {
		return false
	}

	return true
}

// commonPathPrefix returns the longest shared prefix of two paths.
// Used to narrow publisher-side filtering for range watches within one namespace.
func commonPathPrefix(a, b string) string {
	n := min(len(a), len(b))

	for i := range n {
		if a[i] != b[i] {
			return a[:i]
		}
	}

	return a[:n]
}

func eventToProto(ev domain.WatchEvent) *mvccpb.Event {
	key := JoinKey(ev.Namespace, ev.Path)

	kv := &mvccpb.KeyValue{
		Key:         key,
		ModRevision: ev.Revision,
	}

	evType := mvccpb.PUT

	switch ev.Type {
	case domain.EventTypeDeleted:
		evType = mvccpb.DELETE
		// etcd semantics: delete events carry Version=0; ModRevision is the delete revision.
	default:
		if ev.Config != nil {
			kv.Value = []byte(ev.Config.Content)
			kv.CreateRevision = ev.Config.CreateRevision
			kv.ModRevision = ev.Config.Revision
			kv.Version = ev.Config.Version
		}
	}

	return &mvccpb.Event{
		Type: evType,
		Kv:   kv,
	}
}

func changelogToEvent(e *domain.ChangelogEntry, value []byte) *mvccpb.Event {
	key := JoinKey(e.Namespace, e.Path)

	kv := &mvccpb.KeyValue{
		Key:         key,
		ModRevision: e.Revision,
		Version:     e.Version,
		Value:       value,
	}

	evType := mvccpb.PUT
	if e.Type == domain.EventTypeDeleted {
		evType = mvccpb.DELETE
		kv.Value = nil
		kv.Version = 0 // etcd semantics: delete events reset version to 0
	}

	return &mvccpb.Event{
		Type: evType,
		Kv:   kv,
	}
}

func revisionOfEvent(ev domain.WatchEvent) int64 {
	if ev.Revision > 0 {
		return ev.Revision
	}

	if ev.Config != nil {
		return ev.Config.Revision
	}

	return 0
}
