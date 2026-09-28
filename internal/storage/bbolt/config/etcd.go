package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage/internal"
	"github.com/sergeyslonimsky/elara/pkg/bbolt"
)

// GetKVAtRevision returns the value for (namespace, path) as it existed at
// the given revision (looked up in the history bucket). Returns (nil, nil)
// when no history entry exists at or before that revision.
func (r *Repository) GetKVAtRevision(
	ctx context.Context,
	namespace, path string,
	revision int64,
) ([]byte, error) {
	var out []byte

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		history := r.dbm.GetQuerier(ctx).Bucket(bucketHistory)

		val := lookupHistoryAtRevision(history, namespace, path, revision)
		if val != nil {
			out = make([]byte, len(val))
			copy(out, val)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get kv at revision: %w", err)
	}

	return out, nil
}

// CurrentRevisionValue returns the current global revision counter.
func (r *Repository) CurrentRevisionValue(ctx context.Context) (int64, error) {
	return r.CurrentRevision(ctx)
}

// RangeQuery returns key-value pairs in range [startNS/startPath ..
// endNS/endPath). If endNS and endPath are empty, returns only the single
// key at startNS/startPath. If endNS == "\x00" (etcd "all keys >= start"
// convention) scans everything >= start. revision > 0 enables point-in-time
// reads from the history bucket.
func (r *Repository) RangeQuery(
	ctx context.Context,
	startNS, startPath string,
	endNS, endPath string,
	limit int64,
	revision int64,
	keysOnly bool,
) ([]*domain.KVPair, bool, error) {
	var (
		results []*domain.KVPair
		more    bool
	)

	rp := buildRangeParams(startNS, startPath, endNS, endPath)

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)
		metaBkt := q.Bucket(bucketMeta)
		content := q.Bucket(bucketContent)
		history := q.Bucket(bucketHistory)
		codec := bbolt.JSONCodec[internal.ConfigMeta]{}
		c := metaBkt.Cursor()

		for k, v := c.Seek(rp.startKey); k != nil; k, v = c.Next() {
			if shouldBreakRange(k, rp) {
				break
			}

			var m internal.ConfigMeta
			if err := codec.Unmarshal(v, &m); err != nil {
				return fmt.Errorf("unmarshal meta: %w", err)
			}

			ns, path := parseConfigKey(k)

			val, modRev, ok := readKVValue(content, history, k, ns, path, m, revision, keysOnly)
			if !ok {
				continue
			}

			results = append(results, &domain.KVPair{
				Namespace:      ns,
				Path:           path,
				Value:          val,
				CreateRevision: m.CreateRevision,
				ModRevision:    modRev,
				Version:        m.Version,
				Lease:          m.Lease,
			})

			if limit > 0 && int64(len(results)) >= limit {
				if _, next := c.Next(); next != nil {
					more = true
				}

				break
			}
		}

		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("range query: %w", err)
	}

	return results, more, nil
}

// rangeParams holds precomputed range bounds used by RangeQuery and
// DeleteRangeKeys.
type rangeParams struct {
	startKey  []byte
	endKey    []byte
	singleKey bool
	scanAll   bool
}

func buildRangeParams(startNS, startPath, endNS, endPath string) rangeParams {
	singleKey := endNS == "" && endPath == ""
	scanAll := endNS == "\x00"
	startKey := configKey(startNS, startPath)

	var endKey []byte
	if !singleKey && !scanAll {
		endKey = configKey(endNS, endPath)
	}

	return rangeParams{startKey: startKey, endKey: endKey, singleKey: singleKey, scanAll: scanAll}
}

func shouldBreakRange(k []byte, rp rangeParams) bool {
	if rp.singleKey {
		return !bytes.Equal(k, rp.startKey)
	}

	if !rp.scanAll && len(rp.endKey) > 0 {
		return bytes.Compare(k, rp.endKey) >= 0
	}

	return false
}

// readKVValue resolves the value for a KV pair, optionally performing a
// point-in-time historical lookup. Returns (value, modRevision, ok). When ok
// is false the key did not exist at the requested revision and the caller
// should skip the entry.
func readKVValue(
	content, history bbolt.Bucket,
	k []byte, ns, path string,
	m internal.ConfigMeta, revision int64, keysOnly bool,
) ([]byte, int64, bool) {
	if keysOnly {
		return nil, m.Revision, true
	}

	if revision > 0 && revision < m.Revision {
		histVal := lookupHistoryAtRevision(history, ns, path, revision)
		if histVal == nil {
			return nil, 0, false
		}

		return histVal, revision, true
	}

	raw := content.Get(k)
	val := make([]byte, len(raw))
	copy(val, raw)

	return val, m.Revision, true
}

func lookupHistoryAtRevision(history bbolt.Bucket, namespace, path string, revision int64) []byte {
	seekKey := historyKey(namespace, path, revision)

	c := history.Cursor()
	k, v := c.Seek(seekKey)

	prefix := historyPrefix(namespace, path)

	if k == nil || !bytes.HasPrefix(k, prefix) {
		k, v = c.Prev()
	} else if !bytes.Equal(k, seekKey) {
		k, v = c.Prev()
	}

	if k == nil || !bytes.HasPrefix(k, prefix) {
		return nil
	}

	out := make([]byte, len(v))
	copy(out, v)

	return out
}

// existingKeyInfo holds the pre-existing state of a key being upserted.
type existingKeyInfo struct {
	meta          internal.ConfigMeta
	prevValueCopy []byte
}

func resolveExistingKey(q bbolt.Querier, key []byte) (existingKeyInfo, bool, error) {
	m, err := bbolt.Get[internal.ConfigMeta](q, bucketMeta, key)
	if errors.Is(err, bbolt.ErrNotFound) {
		return existingKeyInfo{}, false, nil
	}
	if err != nil {
		return existingKeyInfo{}, false, fmt.Errorf("unmarshal existing meta: %w", err)
	}

	var prevValueCopy []byte
	if prevVal := q.Bucket(bucketContent).Get(key); prevVal != nil {
		prevValueCopy = make([]byte, len(prevVal))
		copy(prevValueCopy, prevVal)
	}

	return existingKeyInfo{meta: m, prevValueCopy: prevValueCopy}, true, nil
}

// buildPutMeta constructs the new ConfigMeta for a Put. When the key already
// existed the version is bumped and create-time metadata is carried forward;
// otherwise a fresh create record is built.
func buildPutMeta(
	path string,
	value []byte,
	revision int64,
	lease int64,
	existing existingKeyInfo,
	found bool,
) (internal.ConfigMeta, domain.EventType) {
	now := time.Now()
	newMeta := internal.ConfigMeta{
		ContentHash: computeHash(value),
		Format:      string(domain.DetectFormatFromPath(path)),
		Revision:    revision,
		Lease:       lease,
		UpdatedAt:   now,
	}

	if found {
		newMeta.Version = existing.meta.Version + 1
		newMeta.CreateRevision = existing.meta.CreateRevision
		newMeta.CreatedAt = existing.meta.CreatedAt
		newMeta.Metadata = existing.meta.Metadata
		newMeta.Locked = existing.meta.Locked

		return newMeta, domain.EventTypeUpdated
	}

	newMeta.Version = 1
	newMeta.CreateRevision = revision
	newMeta.CreatedAt = now

	return newMeta, domain.EventTypeCreated
}

// PutKey creates or updates a key in etcd semantics. Returns the previous KV
// (if it existed) and the new revision. Always upserts — etcd Put has no
// version check.
//
// lease's zero value means "no lease" (see domain.LeaseAssignment), so an
// ordinary write passes it empty. This method stays deliberately dumb about it:
// verifying that the lease exists and is live, and maintaining the lease-to-keys
// index, belong to the usecase that owns the transaction.
func (r *Repository) PutKey(
	ctx context.Context,
	namespace, path string,
	value []byte,
	lease domain.LeaseAssignment,
) (*domain.KVPair, int64, error) {
	var (
		prev   *domain.KVPair
		newRev int64
	)

	err := r.dbm.WithTx(ctx, func(ctx context.Context) error {
		p, rev, err := putKeyTx(r.dbm.GetQuerier(ctx), namespace, path, value, lease)
		prev = p
		newRev = rev

		return err
	})
	if err != nil {
		return nil, 0, fmt.Errorf("put key: %w", err)
	}

	return prev, newRev, nil
}

func putKeyTx(
	q bbolt.Querier,
	namespace, path string,
	value []byte,
	lease domain.LeaseAssignment,
) (*domain.KVPair, int64, error) {
	key := configKey(namespace, path)

	existing, found, err := resolveExistingKey(q, key)
	if err != nil {
		return nil, 0, err
	}

	if err := validateNamespaceUnlocked(q, namespace); err != nil {
		return nil, 0, err
	}

	if err := checkPutAllowed(existing, found, path); err != nil {
		return nil, 0, err
	}

	revision, err := nextRevision(q)
	if err != nil {
		return nil, 0, err
	}

	prev := buildPrevKV(existing, found, namespace, path)
	newMeta, eventType := buildPutMeta(path, value, revision, lease.Resolve(prev), existing, found)

	if err := q.Bucket(bucketContent).Put(key, value); err != nil {
		return nil, 0, fmt.Errorf("put content: %w", err)
	}

	if err := bbolt.Put(q, bucketMeta, key, newMeta); err != nil {
		return nil, 0, fmt.Errorf("put meta: %w", err)
	}

	if err := writeHistory(q, namespace, path, revision, value); err != nil {
		return nil, 0, err
	}

	if err := writeChangelog(q, revision, eventType, path, namespace, newMeta.Version); err != nil {
		return nil, 0, err
	}

	return prev, revision, nil
}

// deleteTarget holds a key copy and its associated KVPair snapshot collected
// during the first (read-only) scan of a delete range.
type deleteTarget struct {
	key []byte
	kv  *domain.KVPair
}

func collectDeleteTargets(
	q bbolt.Querier,
	rp rangeParams,
	returnPrev bool,
) ([]deleteTarget, error) {
	metaBkt := q.Bucket(bucketMeta)
	content := q.Bucket(bucketContent)
	codec := bbolt.JSONCodec[internal.ConfigMeta]{}
	c := metaBkt.Cursor()

	var targets []deleteTarget

	for k, v := c.Seek(rp.startKey); k != nil; k, v = c.Next() {
		if shouldBreakRange(k, rp) {
			break
		}

		keyCopy := make([]byte, len(k))
		copy(keyCopy, k)

		ns, path := parseConfigKey(k)
		kv := &domain.KVPair{Namespace: ns, Path: path}

		var m internal.ConfigMeta
		if err := codec.Unmarshal(v, &m); err != nil {
			return nil, fmt.Errorf("unmarshal meta: %w", err)
		}

		if m.Locked {
			return nil, fmt.Errorf("delete range: %w", domain.NewLockedError(path))
		}

		// Always carried, regardless of returnPrev: the caller needs it to drop
		// the lease's claim on a key it just deleted, not only to answer prev_kv.
		kv.Lease = m.Lease

		if returnPrev {
			kv.CreateRevision = m.CreateRevision
			kv.ModRevision = m.Revision
			kv.Version = m.Version

			if val := content.Get(k); val != nil {
				kv.Value = make([]byte, len(val))
				copy(kv.Value, val)
			}
		}

		targets = append(targets, deleteTarget{key: keyCopy, kv: kv})
	}

	return targets, nil
}

// DeleteRangeKeys deletes keys in range and returns deleted KVPairs and the
// new revision.
func (r *Repository) DeleteRangeKeys(
	ctx context.Context,
	startNS, startPath string,
	endNS, endPath string,
	returnPrev bool,
) ([]*domain.KVPair, int64, error) {
	var (
		deleted []*domain.KVPair
		newRev  int64
	)

	rp := buildRangeParams(startNS, startPath, endNS, endPath)

	err := r.dbm.WithTx(ctx, func(ctx context.Context) error {
		kvs, rev, err := deleteRangeKeysTx(r.dbm.GetQuerier(ctx), rp, startNS, returnPrev)
		deleted = kvs
		newRev = rev

		return err
	})
	if err != nil {
		return nil, 0, fmt.Errorf("delete range keys: %w", err)
	}

	return deleted, newRev, nil
}

func deleteRangeKeysTx(
	q bbolt.Querier,
	rp rangeParams,
	startNS string,
	returnPrev bool,
) ([]*domain.KVPair, int64, error) {
	if err := validateNamespaceUnlocked(q, startNS); err != nil {
		return nil, 0, err
	}

	targets, err := collectDeleteTargets(q, rp, returnPrev)
	if err != nil {
		return nil, 0, err
	}

	if len(targets) == 0 {
		return nil, 0, nil
	}

	revision, err := nextRevision(q)
	if err != nil {
		return nil, 0, err
	}

	kvs, err := applyDeletes(q, targets, revision)
	if err != nil {
		return nil, 0, err
	}

	return kvs, revision, nil
}

// DeleteKeys deletes an explicit set of keys in a single revision and returns
// the deleted KVPairs with that revision.
//
// Distinct from DeleteRangeKeys because the keys a lease holds are an arbitrary
// set rather than a contiguous range, and etcd removes all of a lease's keys in
// one revision. A loop over DeleteRangeKeys would allocate a revision per key
// and emit a separate watch batch for each — observable, and wrong for clients
// that reason about revisions.
//
// A key that is already gone is skipped rather than reported: the expiry sweep
// can race a client that deleted the key itself.
func (r *Repository) DeleteKeys(
	ctx context.Context,
	refs []domain.KeyRef,
	returnPrev bool,
) ([]*domain.KVPair, int64, error) {
	var (
		deleted []*domain.KVPair
		newRev  int64
	)

	err := r.dbm.WithTx(ctx, func(ctx context.Context) error {
		kvs, rev, err := deleteKeysTx(r.dbm.GetQuerier(ctx), refs, returnPrev)
		deleted = kvs
		newRev = rev

		return err
	})
	if err != nil {
		return nil, 0, fmt.Errorf("delete keys: %w", err)
	}

	return deleted, newRev, nil
}

func deleteKeysTx(
	q bbolt.Querier,
	refs []domain.KeyRef,
	returnPrev bool,
) ([]*domain.KVPair, int64, error) {
	targets, err := collectKeyTargets(q, refs, returnPrev)
	if err != nil {
		return nil, 0, err
	}

	if len(targets) == 0 {
		return nil, 0, nil
	}

	revision, err := nextRevision(q)
	if err != nil {
		return nil, 0, err
	}

	kvs, err := applyDeletes(q, targets, revision)
	if err != nil {
		return nil, 0, err
	}

	return kvs, revision, nil
}

// collectKeyTargets resolves an explicit key set into delete targets, skipping
// keys that no longer exist. Each namespace is checked for a lock once, however
// many of its keys the set contains.
func collectKeyTargets(
	q bbolt.Querier,
	refs []domain.KeyRef,
	returnPrev bool,
) ([]deleteTarget, error) {
	content := q.Bucket(bucketContent)
	checkedNS := make(map[string]struct{}, 1)

	var targets []deleteTarget

	for _, ref := range refs {
		if _, done := checkedNS[ref.Namespace]; !done {
			if err := validateNamespaceUnlocked(q, ref.Namespace); err != nil {
				return nil, err
			}

			checkedNS[ref.Namespace] = struct{}{}
		}

		key := configKey(ref.Namespace, ref.Path)

		m, err := bbolt.Get[internal.ConfigMeta](q, bucketMeta, key)
		if errors.Is(err, bbolt.ErrNotFound) {
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("unmarshal meta: %w", err)
		}

		if m.Locked {
			return nil, fmt.Errorf("delete keys: %w", domain.NewLockedError(ref.Path))
		}

		kv := &domain.KVPair{Namespace: ref.Namespace, Path: ref.Path, Lease: m.Lease}

		if returnPrev {
			kv.CreateRevision = m.CreateRevision
			kv.ModRevision = m.Revision
			kv.Version = m.Version

			if val := content.Get(key); val != nil {
				kv.Value = make([]byte, len(val))
				copy(kv.Value, val)
			}
		}

		targets = append(targets, deleteTarget{key: key, kv: kv})
	}

	return targets, nil
}

// applyDeletes removes every target's content and meta and appends one changelog
// entry per key, all at the given revision. Shared by the range and explicit-set
// delete paths so both emit the same records.
func applyDeletes(
	q bbolt.Querier,
	targets []deleteTarget,
	revision int64,
) ([]*domain.KVPair, error) {
	content := q.Bucket(bucketContent)
	metaBkt := q.Bucket(bucketMeta)
	kvs := make([]*domain.KVPair, 0, len(targets))

	for _, t := range targets {
		if err := content.Delete(t.key); err != nil {
			return nil, fmt.Errorf("delete content: %w", err)
		}

		if err := metaBkt.Delete(t.key); err != nil {
			return nil, fmt.Errorf("delete meta: %w", err)
		}

		if err := writeChangelog(q, revision, domain.EventTypeDeleted, t.kv.Path, t.kv.Namespace, 0); err != nil {
			return nil, err
		}

		kvs = append(kvs, t.kv)
	}

	return kvs, nil
}

func buildPrevKV(existing existingKeyInfo, found bool, namespace, path string) *domain.KVPair {
	if !found {
		return nil
	}

	return &domain.KVPair{
		Namespace:      namespace,
		Path:           path,
		Value:          existing.prevValueCopy,
		CreateRevision: existing.meta.CreateRevision,
		ModRevision:    existing.meta.Revision,
		Version:        existing.meta.Version,
		Lease:          existing.meta.Lease,
	}
}

func checkPutAllowed(existing existingKeyInfo, found bool, path string) error {
	if found && existing.meta.Locked {
		return fmt.Errorf("put: %w", domain.NewLockedError(path))
	}

	return nil
}
