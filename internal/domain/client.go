package domain

import "time"

// ConnectionInfo is the metadata captured when a gRPC connection is established.
// Fields populated from standard gRPC metadata (user-agent) and project-specific
// headers (x-client-*).
type ConnectionInfo struct {
	PeerAddress   string
	UserAgent     string
	ClientName    string
	ClientVersion string
	K8sNamespace  string
	K8sPod        string
	K8sNode       string
	InstanceID    string
}

// Client is a live snapshot of a connected (or recently disconnected) client.
//
// Counters, ActiveWatches, and LastActivityAt are managed atomically and may
// change between the moment a snapshot is taken and when it is read.
type Client struct {
	ID              string
	PeerAddress     string
	UserAgent       string
	ClientName      string
	ClientVersion   string
	K8sNamespace    string
	K8sPod          string
	K8sNode         string
	InstanceID      string
	ConnectedAt     time.Time
	DisconnectedAt  *time.Time // nil while connected
	LastActivityAt  time.Time
	ActiveWatches   int32
	ActiveWatchList []ActiveWatch // detail; len() matches ActiveWatches for live clients
	RequestCounts   map[string]int64
	ErrorCount      int64
}

// ActiveWatch is one open Watch held by a client. Keys are stored in their
// etcd-encoded form ("/{namespace}/{path}"); the UI splits them for display.
type ActiveWatch struct {
	WatchID        int64
	StartKey       string // etcd-encoded key (e.g. "/default/foo")
	EndKey         string // empty for single-key watches; "\x00" for "all keys >= start"
	StartRevision  int64
	CreatedAt      time.Time
	PrevKv         bool
	ProgressNotify bool
}

// IsActive reports whether the client is still connected.
func (c *Client) IsActive() bool {
	return c.DisconnectedAt == nil
}

// ClientEvent is an entry in the per-client recent-events ring buffer.
// These events are in-memory only; they are not persisted to history.
type ClientEvent struct {
	Timestamp time.Time
	Method    string
	Key       string
	Revision  int64
	Duration  time.Duration
	Error     string // empty on success
}

// ClientChangeKind enumerates the kinds of notifications emitted by the
// monitor publisher.
type ClientChangeKind int

const (
	// ClientConnected is emitted immediately when a new connection is accepted.
	ClientConnected ClientChangeKind = iota + 1
	// ClientDisconnected is emitted immediately when a connection closes.
	ClientDisconnected
	// ClientActivity is a throttled event indicating one or more RPCs have
	// occurred for this client since the last event. It is emitted at most
	// once per throttle window to avoid flooding global subscribers.
	ClientActivity
	// ClientRequestRecorded is a per-RPC event. Emitted only via the
	// per-client publisher (Registry.SubscribeClient), never globally — there
	// is no throttling for these so the volume can be high.
	ClientRequestRecorded
)

// ClientChange is a single change notification from the monitor publisher.
// Subscribers receive these on a buffered channel and should convert to their
// own protocol (e.g., ConnectRPC WatchClientsResponse / WatchClientResponse).
type ClientChange struct {
	Kind   ClientChangeKind
	Client *Client // a snapshot at the moment of the change
	// Event is populated only for ClientRequestRecorded. Carries the per-RPC
	// detail (method, key, duration, error) so the UI can append to a live
	// activity log without re-querying the ring buffer.
	Event *ClientEvent
}
