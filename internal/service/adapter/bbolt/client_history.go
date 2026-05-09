package bbolt

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

var errNoDisconnectedAt = errors.New("client_history: snapshot has no DisconnectedAt")

// ClientHistoryRepo persists snapshots of disconnected clients.
//
// Key encoding: 8-byte big-endian unix nanoseconds of disconnected_at (so the
// natural cursor order is chronological — oldest first).
type ClientHistoryRepo struct {
	store *Store
}

func NewClientHistoryRepo(store *Store) *ClientHistoryRepo {
	return &ClientHistoryRepo{store: store}
}

type clientHistoryRow struct {
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

// Save persists one client snapshot. The snapshot must have a non-nil
// DisconnectedAt — keys are derived from it.
func (r *ClientHistoryRepo) Save(_ context.Context, c *domain.Client) error {
	if c.DisconnectedAt == nil {
		return errNoDisconnectedAt
	}

	row := clientHistoryRow{
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

	val, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("marshal client history: %w", err)
	}

	err = r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketClientHistory))
		key := historyTimeKey(*c.DisconnectedAt)

		// Disambiguate same-nanosecond keys by appending the conn ID — extremely
		// rare in practice but possible under load.
		if b.Get(key) != nil {
			key = append(key, []byte(c.ID)...)
		}

		if err := b.Put(key, val); err != nil {
			return fmt.Errorf("put client history: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("save client history: %w", err)
	}

	return nil
}

// List returns up to `limit` most-recent client snapshots, newest first.
// limit <= 0 returns all.
func (r *ClientHistoryRepo) List(_ context.Context, limit int) ([]*domain.Client, error) {
	var out []*domain.Client

	err := r.store.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketClientHistory))
		c := b.Cursor()

		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			if limit > 0 && len(out) >= limit {
				break
			}

			var row clientHistoryRow
			if err := json.Unmarshal(v, &row); err != nil {
				return fmt.Errorf("unmarshal client history: %w", err)
			}

			out = append(out, rowToDomain(&row))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list client history: %w", err)
	}

	return out, nil
}

// ListByClient returns historical snapshots filtered by (client_name, k8s_namespace),
// newest first, capped at limit. limit <= 0 returns all matches.
//
// Implementation: full scan of the bucket (O(N)). With max_records ~1000 this
// is fast enough for UI use; if N grows beyond this, switch to a secondary index.
func (r *ClientHistoryRepo) ListByClient(
	_ context.Context,
	clientName, k8sNamespace string,
	limit int,
) ([]*domain.Client, error) {
	var out []*domain.Client

	err := r.store.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketClientHistory))
		c := b.Cursor()

		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			if limit > 0 && len(out) >= limit {
				break
			}

			var row clientHistoryRow
			if err := json.Unmarshal(v, &row); err != nil {
				return fmt.Errorf("unmarshal client history: %w", err)
			}

			if row.ClientName != clientName {
				continue
			}

			if row.K8sNamespace != k8sNamespace {
				continue
			}

			out = append(out, rowToDomain(&row))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list client history by client: %w", err)
	}

	return out, nil
}

// Count returns the number of stored snapshots.
func (r *ClientHistoryRepo) Count(_ context.Context) (int, error) {
	var n int

	err := r.store.db.View(func(tx *bolt.Tx) error {
		stats := tx.Bucket([]byte(bucketClientHistory)).Stats()
		n = stats.KeyN

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("count client history: %w", err)
	}

	return n, nil
}

// DeleteOldest removes up to n oldest entries. Returns the number deleted.
func (r *ClientHistoryRepo) DeleteOldest(_ context.Context, n int) (int, error) {
	if n <= 0 {
		return 0, nil
	}

	deleted := 0

	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketClientHistory))
		c := b.Cursor()

		// Collect keys to delete (mutating during cursor iteration is unsafe).
		var toDelete [][]byte
		for k, _ := c.First(); k != nil && len(toDelete) < n; k, _ = c.Next() {
			keyCopy := make([]byte, len(k))
			copy(keyCopy, k)
			toDelete = append(toDelete, keyCopy)
		}

		for _, k := range toDelete {
			if err := b.Delete(k); err != nil {
				return fmt.Errorf("delete client history: %w", err)
			}
		}

		deleted = len(toDelete)

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("delete oldest client history: %w", err)
	}

	return deleted, nil
}

// DeleteOlderThan removes all entries with disconnected_at < cutoff.
// Returns the number deleted.
func (r *ClientHistoryRepo) DeleteOlderThan(_ context.Context, cutoff time.Time) (int, error) {
	cutoffKey := historyTimeKey(cutoff)
	deleted := 0

	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketClientHistory))
		c := b.Cursor()

		var toDelete [][]byte
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			// Compare only the first 8 bytes (the time portion) — keys may have
			// a conn ID suffix for collision disambiguation.
			if len(k) >= 8 && bytesLess(cutoffKey, k[:8]) {
				break // sorted ascending, no more matches
			}

			if len(k) >= 8 && !bytesLess(k[:8], cutoffKey) {
				break
			}

			keyCopy := make([]byte, len(k))
			copy(keyCopy, k)
			toDelete = append(toDelete, keyCopy)
		}

		for _, k := range toDelete {
			if err := b.Delete(k); err != nil {
				return fmt.Errorf("delete client history: %w", err)
			}
		}

		deleted = len(toDelete)

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("delete client history older than cutoff: %w", err)
	}

	return deleted, nil
}

func historyTimeKey(t time.Time) []byte {
	k := make([]byte, revisionSize)
	binary.BigEndian.PutUint64(k, uint64(t.UnixNano()))

	return k
}

// bytesLess returns true if a < b, treating both as big-endian unsigned integers.
func bytesLess(a, b []byte) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}

	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}

	return false
}

func rowToDomain(r *clientHistoryRow) *domain.Client {
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
