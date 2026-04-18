// Package monitor tracks connected etcd clients and their request activity.
//
// Locking invariant for clientEntry:
//
//	identMu MUST be acquired before countersMu MUST be acquired before watchesMu.
//
// Always release in LIFO order. Atomic fields (lastActivityNanos, activeWatches,
// errorCount) need no locks. Keep this ordering in mind when adding new fields
// or operations that span multiple locks — violating it risks deadlock.
package monitor

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Default knobs — overridable via Config in DI.
const (
	defaultRecentEventsCapacity = 100
	defaultPublisherBuffer      = 256
	defaultActivityThrottle     = 500 * time.Millisecond
)

// Config controls Registry behaviour.
type Config struct {
	// RecentEventsCapacity caps how many events per client are retained in
	// memory. Zero or negative → default (100).
	RecentEventsCapacity int
	// ActivityThrottle is the minimum interval between consecutive ClientActivity
	// notifications for the same client. Zero or negative → default (500ms).
	ActivityThrottle time.Duration
	// DisableActivityEvents suppresses ClientActivity notifications entirely;
	// only Connected/Disconnected events are emitted.
	DisableActivityEvents bool
}

func (c Config) withDefaults() Config {
	if c.RecentEventsCapacity <= 0 {
		c.RecentEventsCapacity = defaultRecentEventsCapacity
	}

	if c.ActivityThrottle <= 0 {
		c.ActivityThrottle = defaultActivityThrottle
	}

	return c
}

// HistorySink receives a snapshot of every disconnected client. Implementations
// must be non-blocking — a slow sink must not slow down disconnect handling.
type HistorySink interface {
	Record(snapshot *domain.Client)
}

// noopHistorySink is the default when no HistorySink is provided.
type noopHistorySink struct{}

func (noopHistorySink) Record(*domain.Client) {}

// Registry is the in-memory source of truth for connected clients.
//
// All operations are safe for concurrent use. The hot path (RecordRequest)
// uses atomic counters and a per-client lock for the events ring buffer to
// avoid global contention.
type Registry struct {
	cfg     Config
	history HistorySink

	clients sync.Map // connID -> *clientEntry
	nextID  atomic.Int64

	pub *publisher
}

// NewRegistry constructs a Registry. cfg is normalized via withDefaults; pass
// the zero value for safe defaults. history may be nil.
func NewRegistry(cfg Config, history HistorySink) *Registry {
	if history == nil {
		history = noopHistorySink{}
	}

	return &Registry{
		cfg:     cfg.withDefaults(),
		history: history,
		pub:     newPublisher(defaultPublisherBuffer),
	}
}

// clientEntry is the internal mutable representation. Public snapshots are
// produced via toDomain() to avoid exposing locks.
type clientEntry struct {
	id          string
	peer        string
	connectedAt time.Time

	// Identity (set lazily on first RPC, then stable).
	identMu       sync.RWMutex
	userAgent     string
	clientName    string
	clientVersion string
	k8sNamespace  string
	k8sPod        string
	k8sNode       string
	instanceID    string
	identSet      bool

	// Hot-path counters — atomic, no lock.
	lastActivityNanos atomic.Int64 // unix nanos
	activeWatches     atomic.Int32
	errorCount        atomic.Int64

	// Per-method counters: protected by countersMu (rarely contended; growth
	// stops once all method names have been seen once).
	countersMu sync.RWMutex
	counters   map[string]*atomic.Int64

	// Detail list of currently-open watches. Keyed by watch ID (assigned by
	// the WatchServer per stream). Updated rarely (on watch create/cancel),
	// so a regular mutex is fine.
	watchesMu sync.RWMutex
	watches   map[int64]domain.ActiveWatch

	events *eventRingBuffer

	// throttling for ClientActivity events
	lastActivityEventNanos atomic.Int64

	// pubMu protects pub assignment. pub is lazily created on the first
	// SubscribeClient call. When non-nil, RecordRequest publishes per-RPC
	// events to it. Cleared and shut down on UnregisterConnection.
	pubMu sync.RWMutex
	pub   *publisher
}

// RegisterConnection records a new connection. info.PeerAddress is required;
// other fields are typically set lazily on the first RPC via UpdateIdentity.
// Returns the assigned connection ID.
func (r *Registry) RegisterConnection(info domain.ConnectionInfo) string {
	id := "conn-" + strconv.FormatInt(r.nextID.Add(1), 10)
	now := time.Now()

	e := &clientEntry{
		id:          id,
		peer:        info.PeerAddress,
		connectedAt: now,
		counters:    make(map[string]*atomic.Int64),
		watches:     make(map[int64]domain.ActiveWatch),
		events:      newEventRingBuffer(r.cfg.RecentEventsCapacity),
	}
	e.lastActivityNanos.Store(now.UnixNano())

	if hasIdentity(info) {
		e.applyIdentity(info)
	}

	r.clients.Store(id, e)

	r.pub.publish(domain.ClientChange{Kind: domain.ClientConnected, Client: e.toDomain()})

	return id
}

// UpdateIdentity fills in client identity fields the first time they are
// observed. Subsequent calls are ignored — identity is considered stable
// for the lifetime of a connection.
func (r *Registry) UpdateIdentity(connID string, info domain.ConnectionInfo) {
	e := r.lookup(connID)
	if e == nil {
		return
	}

	e.identMu.Lock()
	wasUnset := !e.identSet

	if wasUnset {
		e.applyIdentity(info)
	}
	e.identMu.Unlock()
}

// UnregisterConnection marks the connection as closed and forwards a final
// snapshot to the HistorySink. Closes any per-client subscribers so detail-
// view streams exit cleanly.
func (r *Registry) UnregisterConnection(connID string) {
	v, ok := r.clients.LoadAndDelete(connID)
	if !ok {
		return
	}

	e := mustEntry(v)
	snap := e.toDomain()
	snap.DisconnectedAt = new(time.Now())

	r.history.Record(snap)

	// Notify per-client subscribers with the final disconnect event, then close.
	e.pubMu.Lock()
	if e.pub != nil {
		e.pub.publish(domain.ClientChange{Kind: domain.ClientDisconnected, Client: snap})
		e.pub.shutdown()
		e.pub = nil
	}
	e.pubMu.Unlock()

	r.pub.publish(domain.ClientChange{Kind: domain.ClientDisconnected, Client: snap})
}

// RecordRequest is invoked on every completed RPC. It is on the hot path —
// avoid heavy work here.
func (r *Registry) RecordRequest(
	connID, method, key string,
	revision int64,
	duration time.Duration,
	err error,
) {
	e := r.lookup(connID)
	if e == nil {
		return
	}

	now := time.Now()
	e.lastActivityNanos.Store(now.UnixNano())

	if err != nil {
		e.errorCount.Add(1)
	}

	e.incCounter(method)

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	ev := domain.ClientEvent{
		Timestamp: now,
		Method:    method,
		Key:       key,
		Revision:  revision,
		Duration:  duration,
		Error:     errStr,
	}

	e.events.Push(ev)

	r.maybePublishActivity(e, now)

	// Publish per-RPC event to per-client publisher only if it exists
	// (i.e. someone has subscribed via SubscribeClient). The check is a cheap
	// RLock + nil-check so the no-subscribers hot path is fast.
	e.pubMu.RLock()
	pub := e.pub
	e.pubMu.RUnlock()

	if pub != nil {
		pub.publish(domain.ClientChange{
			Kind:   domain.ClientRequestRecorded,
			Client: e.toDomain(),
			Event:  &ev,
		})
	}
}

// RegisterWatch records a newly-opened Watch on a client. The watch ID must
// be unique within the connection. Idempotent — re-registering the same ID
// overwrites the previous detail.
func (r *Registry) RegisterWatch(connID string, w domain.ActiveWatch) {
	e := r.lookup(connID)
	if e == nil {
		return
	}

	e.watchesMu.Lock()
	e.watches[w.WatchID] = w
	count := int32(len(e.watches))
	e.watchesMu.Unlock()

	e.activeWatches.Store(count)
}

// UnregisterWatch removes one watch by ID. No-op if not present.
func (r *Registry) UnregisterWatch(connID string, watchID int64) {
	e := r.lookup(connID)
	if e == nil {
		return
	}

	e.watchesMu.Lock()
	delete(e.watches, watchID)
	count := int32(len(e.watches))
	e.watchesMu.Unlock()

	e.activeWatches.Store(count)
}

// IncActiveWatches and DecActiveWatches remain for callers that don't have
// per-watch detail. They adjust the counter directly without touching the
// detail map. Prefer Register/UnregisterWatch when detail is available.
func (r *Registry) IncActiveWatches(connID string) {
	if e := r.lookup(connID); e != nil {
		e.activeWatches.Add(1)
	}
}

func (r *Registry) DecActiveWatches(connID string) {
	if e := r.lookup(connID); e != nil {
		e.activeWatches.Add(-1)
	}
}

// ActiveWatches returns a copy of one client's open-watch list. Returns nil
// if the client is not (or no longer) connected.
func (r *Registry) ActiveWatches(connID string) []domain.ActiveWatch {
	e := r.lookup(connID)
	if e == nil {
		return nil
	}

	e.watchesMu.RLock()
	defer e.watchesMu.RUnlock()

	if len(e.watches) == 0 {
		return nil
	}

	out := make([]domain.ActiveWatch, 0, len(e.watches))
	for _, w := range e.watches {
		out = append(out, w)
	}

	return out
}

// ListActive returns a snapshot of all currently-connected clients.
func (r *Registry) ListActive() []*domain.Client {
	var out []*domain.Client

	r.clients.Range(func(_, v any) bool {
		out = append(out, mustEntry(v).toDomain())

		return true
	})

	return out
}

// Get returns a snapshot of one connected client, or nil if not found.
func (r *Registry) Get(connID string) *domain.Client {
	e := r.lookup(connID)
	if e == nil {
		return nil
	}

	return e.toDomain()
}

// RecentEvents returns the recent events for one connected client.
// Returns nil if the client is not (or no longer) connected.
func (r *Registry) RecentEvents(connID string) []domain.ClientEvent {
	e := r.lookup(connID)
	if e == nil {
		return nil
	}

	return e.events.Snapshot()
}

// Subscribe returns a buffered channel of change events plus a cleanup func
// that must be called when the subscriber goes away. The channel is closed
// after cleanup runs. Slow subscribers will see events dropped silently.
func (r *Registry) Subscribe() (<-chan domain.ClientChange, func()) {
	return r.pub.subscribe()
}

// SubscribeClient returns a buffered channel of changes affecting a single
// client (per-RPC events, identity updates, eventual disconnect).
//
// If the client is not (or no longer) connected, returns an already-closed
// channel — the caller's loop will exit immediately.
//
// Cleanup must be called when the subscriber goes away.
func (r *Registry) SubscribeClient(connID string) (<-chan domain.ClientChange, func()) {
	e := r.lookup(connID)
	if e == nil {
		closed := make(chan domain.ClientChange)
		close(closed)

		return closed, func() {}
	}

	e.pubMu.Lock()
	if e.pub == nil {
		e.pub = newPublisher(defaultPublisherBuffer)
	}
	pub := e.pub
	e.pubMu.Unlock()

	return pub.subscribe()
}

// Shutdown stops accepting new events and closes all active subscriptions
// (both global and per-client). Already-registered connections remain
// queryable but will not produce any further notifications.
func (r *Registry) Shutdown() {
	r.pub.shutdown()

	r.clients.Range(func(_, v any) bool {
		e := mustEntry(v)

		e.pubMu.Lock()
		if e.pub != nil {
			e.pub.shutdown()
			e.pub = nil
		}
		e.pubMu.Unlock()

		return true
	})
}

// -----------------------------------------------------------------------------
// internal helpers
// -----------------------------------------------------------------------------

func (r *Registry) lookup(connID string) *clientEntry {
	v, ok := r.clients.Load(connID)
	if !ok {
		return nil
	}

	return mustEntry(v)
}

func (r *Registry) maybePublishActivity(e *clientEntry, now time.Time) {
	if r.cfg.DisableActivityEvents {
		return
	}

	throttle := r.cfg.ActivityThrottle

	last := e.lastActivityEventNanos.Load()
	if now.UnixNano()-last < throttle.Nanoseconds() {
		return
	}

	if !e.lastActivityEventNanos.CompareAndSwap(last, now.UnixNano()) {
		return // another goroutine just published
	}

	r.pub.publish(domain.ClientChange{Kind: domain.ClientActivity, Client: e.toDomain()})
}

func (e *clientEntry) applyIdentity(info domain.ConnectionInfo) {
	e.userAgent = info.UserAgent
	e.clientName = info.ClientName
	e.clientVersion = info.ClientVersion
	e.k8sNamespace = info.K8sNamespace
	e.k8sPod = info.K8sPod
	e.k8sNode = info.K8sNode
	e.instanceID = info.InstanceID
	e.identSet = true
}

func (e *clientEntry) incCounter(method string) {
	e.countersMu.RLock()
	c, ok := e.counters[method]
	e.countersMu.RUnlock()

	if ok {
		c.Add(1)

		return
	}

	e.countersMu.Lock()

	if c, ok = e.counters[method]; !ok {
		c = new(atomic.Int64)
		e.counters[method] = c
	}
	e.countersMu.Unlock()

	c.Add(1)
}

func (e *clientEntry) toDomain() *domain.Client {
	e.identMu.RLock()
	identity := domain.Client{
		ID:            e.id,
		PeerAddress:   e.peer,
		UserAgent:     e.userAgent,
		ClientName:    e.clientName,
		ClientVersion: e.clientVersion,
		K8sNamespace:  e.k8sNamespace,
		K8sPod:        e.k8sPod,
		K8sNode:       e.k8sNode,
		InstanceID:    e.instanceID,
		ConnectedAt:   e.connectedAt,
	}
	e.identMu.RUnlock()

	identity.LastActivityAt = time.Unix(0, e.lastActivityNanos.Load())
	identity.ActiveWatches = e.activeWatches.Load()
	identity.ErrorCount = e.errorCount.Load()

	e.countersMu.RLock()
	identity.RequestCounts = make(map[string]int64, len(e.counters))

	for k, v := range e.counters {
		identity.RequestCounts[k] = v.Load()
	}
	e.countersMu.RUnlock()

	e.watchesMu.RLock()
	if len(e.watches) > 0 {
		identity.ActiveWatchList = make([]domain.ActiveWatch, 0, len(e.watches))
		for _, w := range e.watches {
			identity.ActiveWatchList = append(identity.ActiveWatchList, w)
		}
	}
	e.watchesMu.RUnlock()

	return &identity
}

// mustEntry asserts that v (retrieved from the clients sync.Map) is a
// *clientEntry. The sync.Map only stores *clientEntry values by construction,
// so a type mismatch indicates a programming error.
func mustEntry(v any) *clientEntry {
	e, ok := v.(*clientEntry)
	if !ok {
		panic("elara/monitor: sync.Map invariant violated: expected *clientEntry")
	}

	return e
}

func hasIdentity(info domain.ConnectionInfo) bool {
	return info.UserAgent != "" ||
		info.ClientName != "" ||
		info.ClientVersion != "" ||
		info.K8sNamespace != "" ||
		info.K8sPod != "" ||
		info.K8sNode != "" ||
		info.InstanceID != ""
}
