package bbolt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// GetKVAtRevision returns the value for (namespace, path) as it existed at the given
// revision (looking up in the history bucket). Returns nil if no history entry exists
// at or before that revision.
func (r *ConfigRepo) GetKVAtRevision(_ context.Context, namespace, path string, revision int64) ([]byte, error) {
	var out []byte

	err := r.store.db.View(func(tx *bolt.Tx) error {
		history := tx.Bucket([]byte(bucketHistory))

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

// CurrentRevisionValue returns the current global revision.
func (r *ConfigRepo) CurrentRevisionValue(_ context.Context) (int64, error) {
	var rev int64

	err := r.store.db.View(func(tx *bolt.Tx) error {
		sys := tx.Bucket([]byte(bucketSys))
		b := sys.Get([]byte(sysRevisionKey))

		if b != nil {
			rev = parseRevision(b)
		}

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("get current revision: %w", err)
	}

	return rev, nil
}

// RangeQuery returns key-value pairs in range [startNS, startPath) to [endNS, endPath).
// If endNS and endPath are empty, returns only the single key at startNS/startPath.
// If endNS+endPath represents "all keys >= start" (etcd convention with "\0"), scans everything >= start.
// revision > 0 enables point-in-time read from history bucket.
func (r *ConfigRepo) RangeQuery(
	_ context.Context,
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

	err := r.store.db.View(func(tx *bolt.Tx) error {
		metaBkt := tx.Bucket([]byte(bucketMeta))
		content := tx.Bucket([]byte(bucketContent))
		history := tx.Bucket([]byte(bucketHistory))
		c := metaBkt.Cursor()

		for k, v := c.Seek(rp.startKey); k != nil; k, v = c.Next() {
			if shouldBreakRange(k, rp) {
				break
			}

			var m configMeta
			if err := json.Unmarshal(v, &m); err != nil {
				return fmt.Errorf("unmarshal meta: %w", err)
			}

			ns, path := parseConfigKey(k)

			val, modRev, ok := readKVValue(content, history, k, ns, path, &m, revision, keysOnly)
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

// rangeParams holds precomputed range bounds used by RangeQuery and DeleteRangeKeys.
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

// shouldBreakRange returns true when cursor key k is past the requested range.
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
// point-in-time historical lookup. Returns (value, modRevision, ok). When ok is
// false the key did not exist at the requested revision and the caller should
// skip the entry.
func readKVValue(
	content, history *bolt.Bucket,
	k []byte, ns, path string,
	m *configMeta, revision int64, keysOnly bool,
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

func lookupHistoryAtRevision(history *bolt.Bucket, namespace, path string, revision int64) []byte {
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

	// bbolt bytes are only valid inside the enclosing tx — copy before returning.
	out := make([]byte, len(v))
	copy(out, v)

	return out
}

// existingKeyInfo holds the pre-existing state of a key being upserted.
type existingKeyInfo struct {
	meta          *configMeta
	prevValueCopy []byte
}

// resolveExistingKey reads the current meta + value for key. If the key does
// not exist, found is false and info is zero-valued.
func resolveExistingKey(metaBkt, content *bolt.Bucket, key []byte) (existingKeyInfo, bool, error) {
	raw := metaBkt.Get(key)
	if raw == nil {
		return existingKeyInfo{}, false, nil
	}

	var m configMeta
	if err := json.Unmarshal(raw, &m); err != nil {
		return existingKeyInfo{}, false, fmt.Errorf("unmarshal existing meta: %w", err)
	}

	var prevValueCopy []byte
	if prevVal := content.Get(key); prevVal != nil {
		prevValueCopy = make([]byte, len(prevVal))
		copy(prevValueCopy, prevVal)
	}

	return existingKeyInfo{meta: &m, prevValueCopy: prevValueCopy}, true, nil
}

// buildPutMeta constructs the new configMeta for a Put. When the key already
// existed the version is bumped and create-time metadata is carried forward;
// otherwise a fresh create record is built.
func buildPutMeta(
	path string,
	value []byte,
	revision int64,
	existing existingKeyInfo,
	found bool,
) (*configMeta, domain.EventType) {
	now := time.Now()
	newMeta := &configMeta{
		ContentHash: computeHash(value),
		Format:      string(domain.DetectFormatFromPath(path)),
		Revision:    revision,
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

// PutKey creates or updates a key in etcd semantics. Returns previous KV (if existed) and new revision.
// Always upserts (no version check — etcd Put is always an upsert).
func (r *ConfigRepo) PutKey(
	_ context.Context,
	namespace, path string,
	value []byte,
) (*domain.KVPair, int64, error) {
	var (
		prev   *domain.KVPair
		newRev int64
	)

	if err := r.store.db.Update(func(tx *bolt.Tx) error {
		p, rev, err := putKeyTx(tx, namespace, path, value)
		prev = p
		newRev = rev

		return err
	}); err != nil {
		return nil, 0, fmt.Errorf("put key: %w", err)
	}

	return prev, newRev, nil
}

func putKeyTx(tx *bolt.Tx, namespace, path string, value []byte) (*domain.KVPair, int64, error) {
	metaBkt := tx.Bucket([]byte(bucketMeta))
	content := tx.Bucket([]byte(bucketContent))
	key := configKey(namespace, path)

	existing, found, err := resolveExistingKey(metaBkt, content, key)
	if err != nil {
		return nil, 0, err
	}

	if err := validateNamespaceUnlocked(tx, namespace); err != nil {
		return nil, 0, fmt.Errorf("validate namespace unlocked: %w", err)
	}

	if err := checkPutAllowed(existing, found, path); err != nil {
		return nil, 0, err
	}

	revision, err := nextRevision(tx)
	if err != nil {
		return nil, 0, err
	}

	newMeta, eventType := buildPutMeta(path, value, revision, existing, found)
	prev := buildPrevKV(existing, found, namespace, path)

	if err := content.Put(key, value); err != nil {
		return nil, 0, fmt.Errorf("put content: %w", err)
	}

	newMetaBytes, err := json.Marshal(newMeta)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal meta: %w", err)
	}

	if err := metaBkt.Put(key, newMetaBytes); err != nil {
		return nil, 0, fmt.Errorf("put meta: %w", err)
	}

	if err := writeHistory(tx, namespace, path, revision, value); err != nil {
		return nil, 0, err
	}

	if err := writeChangelog(tx, revision, eventType, path, namespace, newMeta.Version); err != nil {
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

// collectDeleteTargets scans the meta bucket for keys in range and returns
// copies of both the raw key and the domain KVPair (with optional value copy).
// Returns an error if any target config is locked.
func collectDeleteTargets(
	metaBkt, content *bolt.Bucket,
	rp rangeParams,
	returnPrev bool,
) ([]deleteTarget, error) {
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

		var m configMeta
		if err := json.Unmarshal(v, &m); err != nil {
			return nil, fmt.Errorf("unmarshal meta: %w", err)
		}

		if m.Locked {
			return nil, fmt.Errorf("delete range: %w", domain.NewLockedError(path))
		}

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

// DeleteRangeKeys deletes keys in range and returns deleted KVPairs and new revision.
func (r *ConfigRepo) DeleteRangeKeys(
	_ context.Context,
	startNS, startPath string,
	endNS, endPath string,
	returnPrev bool,
) ([]*domain.KVPair, int64, error) {
	var (
		deleted []*domain.KVPair
		newRev  int64
	)

	rp := buildRangeParams(startNS, startPath, endNS, endPath)

	if err := r.store.db.Update(func(tx *bolt.Tx) error {
		kvs, rev, err := deleteRangeKeysTx(tx, rp, startNS, returnPrev)
		deleted = kvs
		newRev = rev

		return err
	}); err != nil {
		return nil, 0, fmt.Errorf("delete range keys: %w", err)
	}

	return deleted, newRev, nil
}

func deleteRangeKeysTx(tx *bolt.Tx, rp rangeParams, startNS string, returnPrev bool) ([]*domain.KVPair, int64, error) {
	if err := validateNamespaceUnlocked(tx, startNS); err != nil {
		return nil, 0, fmt.Errorf("validate namespace unlocked: %w", err)
	}

	metaBkt := tx.Bucket([]byte(bucketMeta))
	content := tx.Bucket([]byte(bucketContent))

	targets, err := collectDeleteTargets(metaBkt, content, rp, returnPrev)
	if err != nil {
		return nil, 0, err
	}

	if len(targets) == 0 {
		return nil, 0, nil
	}

	revision, err := nextRevision(tx)
	if err != nil {
		return nil, 0, err
	}

	kvs := make([]*domain.KVPair, 0, len(targets))

	for _, t := range targets {
		if err := content.Delete(t.key); err != nil {
			return nil, 0, fmt.Errorf("delete content: %w", err)
		}

		if err := metaBkt.Delete(t.key); err != nil {
			return nil, 0, fmt.Errorf("delete meta: %w", err)
		}

		if err := writeChangelog(tx, revision, domain.EventTypeDeleted, t.kv.Path, t.kv.Namespace, 0); err != nil {
			return nil, 0, err
		}

		kvs = append(kvs, t.kv)
	}

	return kvs, revision, nil
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
	}
}

func checkPutAllowed(existing existingKeyInfo, found bool, path string) error {
	if found && existing.meta.Locked {
		return fmt.Errorf("put: %w", domain.NewLockedError(path))
	}

	return nil
}
