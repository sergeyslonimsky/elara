// Package client_history is the bbolt-backed repository for disconnected
// client snapshots used by the connected-clients monitor.
//
// Methods that traverse the bucket via a Cursor (List, ListByClient,
// DeleteOldest, DeleteOlderThan) REQUIRE an outer transaction in ctx —
// bbolt cursors are bound to a transaction and the pkg/bbolt autoQuerier
// returns an empty cursor when ctx carries none. Callers (typically the
// monitor service) MUST wrap these calls in Manager.WithTx or WithReadTx
// to obtain a tx-backed querier. The same rule applies to multi-key
// mutations (Save races, DeleteOldest, DeleteOlderThan): atomicity is the
// caller's responsibility.
package client_history

import (
	"context"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage/internal"
	"github.com/sergeyslonimsky/elara/pkg/bbolt"
)

const bucketName = "client_history"

type Repository struct {
	dbm bbolt.Manager
}

func NewRepository(dbm bbolt.Manager) *Repository {
	return &Repository{dbm: dbm}
}

// Save persists one client snapshot. Caller MUST set c.DisconnectedAt — the
// repo derives the key from it.
//
// Same-nanosecond writes are disambiguated by appending the connection ID
// to the key. The get+put is NOT atomic without a caller-side
// Manager.WithTx; two concurrent Save calls for the same nanosecond can
// race on key derivation.
func (r *Repository) Save(ctx context.Context, c *domain.Client) error {
	row := internal.DomainToClientHistoryRow(c)

	codec := bbolt.JSONCodec[internal.ClientHistoryRow]{}
	val, err := codec.Marshal(row)
	if err != nil {
		return fmt.Errorf("save: %w", err)
	}

	bucket := r.dbm.GetQuerier(ctx).Bucket(bucketName)

	key := timeKey(*c.DisconnectedAt)
	if bucket.Get(key) != nil {
		key = append(key, []byte(c.ID)...)
	}

	if err := bucket.Put(key, val); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	return nil
}

// List returns up to `limit` most-recent snapshots, newest first.
// limit <= 0 returns all.
//
// Caller MUST wrap in Manager.WithReadTx — Cursor needs a tx-backed querier.
func (r *Repository) List(ctx context.Context, limit int) ([]*domain.Client, error) {
	var out []*domain.Client

	c := r.dbm.GetQuerier(ctx).Bucket(bucketName).Cursor()

	codec := bbolt.JSONCodec[internal.ClientHistoryRow]{}
	for k, v := c.Last(); k != nil; k, v = c.Prev() {
		if limit > 0 && len(out) >= limit {
			break
		}

		var row internal.ClientHistoryRow
		if err := codec.Unmarshal(v, &row); err != nil {
			return nil, fmt.Errorf("list: decode: %w", err)
		}

		out = append(out, internal.ClientHistoryRowToDomain(row))
	}

	return out, nil
}

// ListByClient returns historical snapshots filtered by (client_name,
// k8s_namespace), newest first, capped at limit. limit <= 0 returns all
// matches.
//
// Implementation: full bucket scan (O(N)). With max_records ~1000 this is
// fast enough for UI use; if N grows beyond that, switch to a secondary
// index. Caller MUST wrap in Manager.WithReadTx.
func (r *Repository) ListByClient(
	ctx context.Context,
	clientName, k8sNamespace string,
	limit int,
) ([]*domain.Client, error) {
	var out []*domain.Client

	c := r.dbm.GetQuerier(ctx).Bucket(bucketName).Cursor()

	codec := bbolt.JSONCodec[internal.ClientHistoryRow]{}
	for k, v := c.Last(); k != nil; k, v = c.Prev() {
		if limit > 0 && len(out) >= limit {
			break
		}

		var row internal.ClientHistoryRow
		if err := codec.Unmarshal(v, &row); err != nil {
			return nil, fmt.Errorf("list by client: decode: %w", err)
		}

		if row.ClientName != clientName || row.K8sNamespace != k8sNamespace {
			continue
		}

		out = append(out, internal.ClientHistoryRowToDomain(row))
	}

	return out, nil
}

// Count returns the number of stored snapshots.
//
// Implementation: O(N) bucket scan. bbolt's bucket Stats() is O(1) but not
// exposed via the Querier abstraction (it is backend-specific). At the
// retention cap (~1000) the iteration cost is negligible.
func (r *Repository) Count(ctx context.Context) (int, error) {
	var n int

	err := r.dbm.GetQuerier(ctx).Bucket(bucketName).ForEach(func(_, _ []byte) error {
		n++

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}

	return n, nil
}

// DeleteOldest removes up to n oldest snapshots. Returns the number deleted.
//
// Cursor traversal + delete — caller MUST wrap in Manager.WithTx.
func (r *Repository) DeleteOldest(ctx context.Context, n int) (int, error) {
	if n <= 0 {
		return 0, nil
	}

	bucket := r.dbm.GetQuerier(ctx).Bucket(bucketName)
	c := bucket.Cursor()

	// Mutating during cursor iteration is unsafe — collect first, then delete.
	var toDelete [][]byte
	for k, _ := c.First(); k != nil && len(toDelete) < n; k, _ = c.Next() {
		keyCopy := make([]byte, len(k))
		copy(keyCopy, k)
		toDelete = append(toDelete, keyCopy)
	}

	for _, k := range toDelete {
		if err := bucket.Delete(k); err != nil {
			return 0, fmt.Errorf("delete oldest: %w", err)
		}
	}

	return len(toDelete), nil
}

// DeleteOlderThan removes all snapshots with disconnected_at < cutoff.
// Returns the number deleted.
//
// Cursor traversal + delete — caller MUST wrap in Manager.WithTx.
func (r *Repository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int, error) {
	cutoffKey := timeKey(cutoff)

	bucket := r.dbm.GetQuerier(ctx).Bucket(bucketName)
	c := bucket.Cursor()

	var toDelete [][]byte
	for k, _ := c.First(); k != nil; k, _ = c.Next() {
		// Compare only the first 8 bytes (the time portion) — keys may have
		// a conn-ID suffix from collision disambiguation in Save.
		if len(k) < timeKeySize {
			continue
		}

		timePart := k[:timeKeySize]

		if !bytesLess(timePart, cutoffKey) {
			// timePart >= cutoff — keys are sorted ascending so we are
			// past all candidates.
			break
		}

		keyCopy := make([]byte, len(k))
		copy(keyCopy, k)
		toDelete = append(toDelete, keyCopy)
	}

	for _, k := range toDelete {
		if err := bucket.Delete(k); err != nil {
			return 0, fmt.Errorf("delete older than cutoff: %w", err)
		}
	}

	return len(toDelete), nil
}
