package internal

import (
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// ClientHistoryRow is the on-disk JSON shape for a disconnected-client
// snapshot. Backend repositories serialize a row per snapshot keyed by
// disconnected-at time (encoding rules live in each backend package).
type ClientHistoryRow struct {
	ID             string           `json:"id"`
	PeerAddress    string           `json:"peer_address"`
	UserAgent      string           `json:"user_agent,omitempty"`
	ClientName     string           `json:"client_name,omitempty"`
	ClientVersion  string           `json:"client_version,omitempty"`
	K8sNamespace   string           `json:"k8s_namespace,omitempty"`
	K8sPod         string           `json:"k8s_pod,omitempty"`
	K8sNode        string           `json:"k8s_node,omitempty"`
	InstanceID     string           `json:"instance_id,omitempty"`
	ConnectedAt    time.Time        `json:"connected_at"`
	DisconnectedAt time.Time        `json:"disconnected_at"`
	LastActivityAt time.Time        `json:"last_activity_at"`
	ActiveWatches  int32            `json:"active_watches"`
	RequestCounts  map[string]int64 `json:"request_counts,omitempty"`
	ErrorCount     int64            `json:"error_count"`
}

// DomainToClientHistoryRow projects a domain client snapshot into the
// row DTO. Callers MUST ensure c.DisconnectedAt is non-nil — this function
// dereferences it without checking.
func DomainToClientHistoryRow(c *domain.Client) ClientHistoryRow {
	return ClientHistoryRow{
		ID:             c.ID,
		PeerAddress:    c.PeerAddress,
		UserAgent:      c.UserAgent,
		ClientName:     c.ClientName,
		ClientVersion:  c.ClientVersion,
		K8sNamespace:   c.K8sNamespace,
		K8sPod:         c.K8sPod,
		K8sNode:        c.K8sNode,
		InstanceID:     c.InstanceID,
		ConnectedAt:    c.ConnectedAt,
		DisconnectedAt: *c.DisconnectedAt,
		LastActivityAt: c.LastActivityAt,
		ActiveWatches:  c.ActiveWatches,
		RequestCounts:  c.RequestCounts,
		ErrorCount:     c.ErrorCount,
	}
}

func ClientHistoryRowToDomain(r ClientHistoryRow) *domain.Client {
	return &domain.Client{
		ID:             r.ID,
		PeerAddress:    r.PeerAddress,
		UserAgent:      r.UserAgent,
		ClientName:     r.ClientName,
		ClientVersion:  r.ClientVersion,
		K8sNamespace:   r.K8sNamespace,
		K8sPod:         r.K8sPod,
		K8sNode:        r.K8sNode,
		InstanceID:     r.InstanceID,
		ConnectedAt:    r.ConnectedAt,
		DisconnectedAt: new(r.DisconnectedAt),
		LastActivityAt: r.LastActivityAt,
		ActiveWatches:  r.ActiveWatches,
		RequestCounts:  r.RequestCounts,
		ErrorCount:     r.ErrorCount,
	}
}
