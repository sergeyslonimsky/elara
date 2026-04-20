package bbolt

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type ConfigRepo struct {
	store *Store
}

func NewConfigRepo(store *Store) *ConfigRepo {
	return &ConfigRepo{store: store}
}

// Create creates a new config entry. Writes to content, meta, history, and changelog buckets atomically.
func (r *ConfigRepo) Create(_ context.Context, cfg *domain.Config) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte(bucketMeta))
		key := configKey(cfg.Namespace, cfg.Path)

		if meta.Get(key) != nil {
			return domain.NewAlreadyExistsError("config", cfg.Path)
		}

		cfg.GenerateHash()
		cfg.SetDefaults()

		now := time.Now()
		cfg.Version = 1
		cfg.CreatedAt = now
		cfg.UpdatedAt = now

		revision, err := nextRevision(tx)
		if err != nil {
			return err
		}

		cfg.Revision = revision
		cfg.CreateRevision = revision

		if err := tx.Bucket([]byte(bucketContent)).Put(key, []byte(cfg.Content)); err != nil {
			return fmt.Errorf("put content: %w", err)
		}

		metaBytes, err := json.Marshal(domainToConfigMeta(cfg))
		if err != nil {
			return fmt.Errorf("marshal meta: %w", err)
		}

		if err := meta.Put(key, metaBytes); err != nil {
			return fmt.Errorf("put meta: %w", err)
		}

		if err := writeHistory(tx, cfg.Namespace, cfg.Path, revision, []byte(cfg.Content)); err != nil {
			return err
		}

		return writeChangelog(tx, revision, domain.EventTypeCreated, cfg.Path, cfg.Namespace, cfg.Version)
	})
	if err != nil {
		return fmt.Errorf("create config: %w", err)
	}

	return nil
}

// Get retrieves a config by path and namespace.
func (r *ConfigRepo) Get(_ context.Context, path, namespace string) (*domain.Config, error) {
	var cfg *domain.Config

	err := r.store.db.View(func(tx *bolt.Tx) error {
		key := configKey(namespace, path)

		metaBytes := tx.Bucket([]byte(bucketMeta)).Get(key)
		if metaBytes == nil {
			return domain.NewNotFoundError("config", path)
		}

		var m configMeta
		if err := json.Unmarshal(metaBytes, &m); err != nil {
			return fmt.Errorf("unmarshal meta: %w", err)
		}

		content := tx.Bucket([]byte(bucketContent)).Get(key)

		cfg = configMetaToDomain(&m, string(content), path, namespace)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}

	return cfg, nil
}

// Update updates a config with optimistic locking on version.
func (r *ConfigRepo) Update(_ context.Context, cfg *domain.Config) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte(bucketMeta))
		key := configKey(cfg.Namespace, cfg.Path)

		metaBytes := meta.Get(key)
		if metaBytes == nil {
			return domain.NewNotFoundError("config", cfg.Path)
		}

		var existing configMeta
		if err := json.Unmarshal(metaBytes, &existing); err != nil {
			return fmt.Errorf("unmarshal existing meta: %w", err)
		}

		if existing.Version != cfg.Version {
			return domain.NewConflictError(cfg.Version, existing.Version)
		}

		cfg.GenerateHash()

		now := time.Now()
		cfg.Version = existing.Version + 1
		cfg.CreatedAt = existing.CreatedAt
		cfg.UpdatedAt = now
		cfg.CreateRevision = existing.CreateRevision

		revision, err := nextRevision(tx)
		if err != nil {
			return err
		}

		cfg.Revision = revision

		if err := tx.Bucket([]byte(bucketContent)).Put(key, []byte(cfg.Content)); err != nil {
			return fmt.Errorf("put content: %w", err)
		}

		newMetaBytes, err := json.Marshal(domainToConfigMeta(cfg))
		if err != nil {
			return fmt.Errorf("marshal meta: %w", err)
		}

		if err := meta.Put(key, newMetaBytes); err != nil {
			return fmt.Errorf("put meta: %w", err)
		}

		if err := writeHistory(tx, cfg.Namespace, cfg.Path, revision, []byte(cfg.Content)); err != nil {
			return err
		}

		return writeChangelog(tx, revision, domain.EventTypeUpdated, cfg.Path, cfg.Namespace, cfg.Version)
	})
	if err != nil {
		return fmt.Errorf("update config: %w", err)
	}

	return nil
}

// Delete removes a config by path and namespace.
func (r *ConfigRepo) Delete(_ context.Context, path, namespace string) (int64, error) {
	var newRev int64

	err := r.store.db.Update(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte(bucketMeta))
		key := configKey(namespace, path)

		if meta.Get(key) == nil {
			return domain.NewNotFoundError("config", path)
		}

		revision, err := nextRevision(tx)
		if err != nil {
			return err
		}

		if err := tx.Bucket([]byte(bucketContent)).Delete(key); err != nil {
			return fmt.Errorf("delete content: %w", err)
		}

		if err := meta.Delete(key); err != nil {
			return fmt.Errorf("delete meta: %w", err)
		}

		if err := writeChangelog(tx, revision, domain.EventTypeDeleted, path, namespace, 0); err != nil {
			return err
		}

		newRev = revision

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("delete config: %w", err)
	}

	return newRev, nil
}

// ListByPrefix returns all configs matching the given path prefix and namespace.
func (r *ConfigRepo) ListByPrefix(_ context.Context, pathPrefix, namespace string) ([]*domain.Config, error) {
	var configs []*domain.Config

	err := r.store.db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte(bucketMeta))
		content := tx.Bucket([]byte(bucketContent))

		return scanMeta(meta, namespace, pathPrefix, func(key []byte, m *configMeta) error {
			ns, path := parseConfigKey(key)
			contentBytes := content.Get(key)
			configs = append(configs, configMetaToDomain(m, string(contentBytes), path, ns))

			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("list configs by prefix: %w", err)
	}

	return configs, nil
}

// ListAllByNamespace returns every config in the given namespace.
func (r *ConfigRepo) ListAllByNamespace(ctx context.Context, namespace string) ([]*domain.Config, error) {
	return r.ListByPrefix(ctx, "", namespace)
}

// ListSummariesByPrefix returns summaries (without content) for all configs matching the prefix.
func (r *ConfigRepo) ListSummariesByPrefix(
	_ context.Context,
	pathPrefix, namespace string,
) ([]*domain.ConfigSummary, error) {
	var summaries []*domain.ConfigSummary

	err := r.store.db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte(bucketMeta))

		return scanMeta(meta, namespace, pathPrefix, func(key []byte, m *configMeta) error {
			ns, path := parseConfigKey(key)
			summaries = append(summaries, configMetaToSummary(m, path, ns))

			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("list config summaries by prefix: %w", err)
	}

	return summaries, nil
}

// ListSummaryPage returns a paginated page of config summaries.
func (r *ConfigRepo) ListSummaryPage(
	_ context.Context,
	pathPrefix, namespace string,
	limit, offset int,
) ([]*domain.ConfigSummary, int, error) {
	var (
		summaries []*domain.ConfigSummary
		total     int
	)

	err := r.store.db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte(bucketMeta))
		idx := 0

		return scanMeta(meta, namespace, pathPrefix, func(key []byte, m *configMeta) error {
			if idx >= offset && len(summaries) < limit {
				ns, path := parseConfigKey(key)
				summaries = append(summaries, configMetaToSummary(m, path, ns))
			}

			idx++
			total = idx

			return nil
		})
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list config summary page: %w", err)
	}

	return summaries, total, nil
}

// ListConfigPage returns a paginated page of full configs.
func (r *ConfigRepo) ListConfigPage(
	_ context.Context,
	pathPrefix, namespace string,
	limit, offset int,
) ([]*domain.Config, int, error) {
	var (
		configs []*domain.Config
		total   int
	)

	err := r.store.db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte(bucketMeta))
		content := tx.Bucket([]byte(bucketContent))
		idx := 0

		return scanMeta(meta, namespace, pathPrefix, func(key []byte, m *configMeta) error {
			if idx >= offset && len(configs) < limit {
				ns, path := parseConfigKey(key)
				contentBytes := content.Get(key)
				configs = append(configs, configMetaToDomain(m, string(contentBytes), path, ns))
			}

			idx++
			total = idx

			return nil
		})
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list config page: %w", err)
	}

	return configs, total, nil
}

// CountByNamespace returns the number of configs in a namespace.
func (r *ConfigRepo) CountByNamespace(_ context.Context, namespace string) (int, error) {
	var count int

	err := r.store.db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte(bucketMeta))

		return scanMeta(meta, namespace, "/", func(_ []byte, _ *configMeta) error {
			count++

			return nil
		})
	})
	if err != nil {
		return 0, fmt.Errorf("count configs by namespace: %w", err)
	}

	return count, nil
}

// SearchByPath searches for configs whose path contains the query string (case-insensitive).
// SearchByPath returns all configs whose path contains the query (case-insensitive).
// Sorting and pagination is done in the usecase layer.
func (r *ConfigRepo) SearchByPath(
	_ context.Context,
	query, namespace string,
) ([]*domain.ConfigSummary, error) {
	var results []*domain.ConfigSummary

	queryLower := strings.ToLower(query)

	err := r.store.db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte(bucketMeta))
		c := meta.Cursor()

		var k, v []byte

		var prefix []byte
		if namespace != "" {
			prefix = configKeyPrefix(namespace, "")
			k, v = c.Seek(prefix)
		} else {
			k, v = c.First()
		}

		for ; k != nil; k, v = c.Next() {
			if namespace != "" {
				if !bytes.HasPrefix(k, prefix) {
					break
				}
			}

			ns, path := parseConfigKey(k)
			if !strings.Contains(strings.ToLower(path), queryLower) {
				continue
			}

			var m configMeta
			if err := json.Unmarshal(v, &m); err != nil {
				return fmt.Errorf("unmarshal meta: %w", err)
			}

			results = append(results, configMetaToSummary(&m, path, ns))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("search configs by path: %w", err)
	}

	return results, nil
}

// GetConfigHistory returns the last `limit` history entries for a config, newest first.
// Cross-references history bucket (content) with changelog bucket (event type, timestamp).
func (r *ConfigRepo) GetConfigHistory(
	_ context.Context,
	path, namespace string,
	limit int,
) ([]*domain.HistoryEntry, error) {
	var entries []*domain.HistoryEntry

	err := r.store.db.View(func(tx *bolt.Tx) error {
		history := tx.Bucket([]byte(bucketHistory))
		changelog := tx.Bucket([]byte(bucketChangelog))
		prefix := historyPrefix(namespace, path)

		// Collect all matching keys.
		var keys [][]byte

		c := history.Cursor()
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			keyCopy := make([]byte, len(k))
			copy(keyCopy, k)
			keys = append(keys, keyCopy)
		}

		start := max(len(keys)-limit, 0)

		// Reverse order: newest first.
		for i := len(keys) - 1; i >= start; i-- {
			content := history.Get(keys[i])
			revBytes := keys[i][len(prefix):]
			rev := parseRevision(revBytes)

			entry := &domain.HistoryEntry{
				Revision:    rev,
				Content:     string(content),
				ContentHash: computeHash(content),
			}

			// Look up changelog for event type and timestamp.
			if clData := changelog.Get(revisionBytes(rev)); clData != nil {
				var cl changelogEntry
				if err := json.Unmarshal(clData, &cl); err == nil {
					entry.EventType = domain.EventType(cl.Type)
					entry.Timestamp = cl.Timestamp
				}
			}

			entries = append(entries, entry)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get config history: %w", err)
	}

	return entries, nil
}

// GetAtRevision returns the history entry at a specific revision (or the closest earlier one).
func (r *ConfigRepo) GetAtRevision(
	_ context.Context,
	path, namespace string,
	revision int64,
) (*domain.HistoryEntry, error) {
	var entry *domain.HistoryEntry

	err := r.store.db.View(func(tx *bolt.Tx) error {
		history := tx.Bucket([]byte(bucketHistory))
		changelog := tx.Bucket([]byte(bucketChangelog))
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
			return domain.NewNotFoundError("config history", path)
		}

		revBytes := k[len(prefix):]
		rev := parseRevision(revBytes)

		entry = &domain.HistoryEntry{
			Revision:    rev,
			Content:     string(v),
			ContentHash: computeHash(v),
		}

		if clData := changelog.Get(revisionBytes(rev)); clData != nil {
			var cl changelogEntry
			if err := json.Unmarshal(clData, &cl); err == nil {
				entry.EventType = domain.EventType(cl.Type)
				entry.Timestamp = cl.Timestamp
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get config at revision: %w", err)
	}

	return entry, nil
}

// CurrentRevision returns the current global revision number.
func (r *ConfigRepo) CurrentRevision(_ context.Context) (int64, error) {
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

// ListChanges returns changelog entries since the given revision.
func (r *ConfigRepo) ListChanges(
	_ context.Context,
	sinceRevision int64,
	limit int,
) ([]*domain.ChangelogEntry, error) {
	var entries []*domain.ChangelogEntry

	err := r.store.db.View(func(tx *bolt.Tx) error {
		changelog := tx.Bucket([]byte(bucketChangelog))
		seekKey := revisionBytes(sinceRevision + 1)

		c := changelog.Cursor()
		for k, v := c.Seek(seekKey); k != nil && len(entries) < limit; k, v = c.Next() {
			var e changelogEntry
			if err := json.Unmarshal(v, &e); err != nil {
				return fmt.Errorf("unmarshal changelog entry: %w", err)
			}

			rev := parseRevision(k)
			entries = append(entries, changelogEntryToDomain(&e, rev))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list changes: %w", err)
	}

	return entries, nil
}

// ListRecentChanges returns the most recent changelog entries, newest first.
func (r *ConfigRepo) ListRecentChanges(
	_ context.Context,
	limit int,
) ([]*domain.ChangelogEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	var entries []*domain.ChangelogEntry

	err := r.store.db.View(func(tx *bolt.Tx) error {
		changelog := tx.Bucket([]byte(bucketChangelog))
		c := changelog.Cursor()

		for k, v := c.Last(); k != nil && len(entries) < limit; k, v = c.Prev() {
			var e changelogEntry
			if err := json.Unmarshal(v, &e); err != nil {
				return fmt.Errorf("unmarshal changelog entry: %w", err)
			}

			rev := parseRevision(k)
			entries = append(entries, changelogEntryToDomain(&e, rev))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list recent changes: %w", err)
	}

	return entries, nil
}

// --- internal helpers ---

func computeHash(data []byte) string {
	h := sha256.Sum256(data)

	return hex.EncodeToString(h[:])
}

func nextRevision(tx *bolt.Tx) (int64, error) {
	sys := tx.Bucket([]byte(bucketSys))
	current := parseRevision(sys.Get([]byte(sysRevisionKey)))
	next := current + 1

	if err := sys.Put([]byte(sysRevisionKey), revisionBytes(next)); err != nil {
		return 0, fmt.Errorf("update revision: %w", err)
	}

	return next, nil
}

func writeHistory(tx *bolt.Tx, namespace, path string, revision int64, content []byte) error {
	history := tx.Bucket([]byte(bucketHistory))

	if err := history.Put(historyKey(namespace, path, revision), content); err != nil {
		return fmt.Errorf("put history: %w", err)
	}

	return nil
}

func writeChangelog(
	tx *bolt.Tx,
	revision int64,
	eventType domain.EventType,
	path, namespace string,
	version int64,
) error {
	changelog := tx.Bucket([]byte(bucketChangelog))

	entry := changelogEntry{
		Type:      int(eventType),
		Path:      path,
		Namespace: namespace,
		Version:   version,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal changelog entry: %w", err)
	}

	if err := changelog.Put(revisionBytes(revision), data); err != nil {
		return fmt.Errorf("put changelog: %w", err)
	}

	return nil
}

// shouldSkipByPath returns true when the key should be skipped because it does
// not match a cross-namespace path prefix filter.
func shouldSkipByPath(k []byte, namespace, pathPrefix string) bool {
	if namespace != "" || pathPrefix == "" || pathPrefix == "/" {
		return false
	}

	_, path := parseConfigKey(k)

	return !strings.HasPrefix(path, pathPrefix)
}

// scanMeta iterates over the meta bucket, optionally filtered by namespace and path prefix.
func scanMeta(
	meta *bolt.Bucket,
	namespace, pathPrefix string,
	fn func(key []byte, m *configMeta) error,
) error {
	c := meta.Cursor()
	prefix := configKeyPrefix(namespace, pathPrefix)

	var k, v []byte
	if prefix != nil {
		k, v = c.Seek(prefix)
	} else {
		k, v = c.First()
	}

	for ; k != nil; k, v = c.Next() {
		if prefix != nil && !bytes.HasPrefix(k, prefix) {
			break
		}

		if shouldSkipByPath(k, namespace, pathPrefix) {
			continue
		}

		var m configMeta
		if err := json.Unmarshal(v, &m); err != nil {
			return fmt.Errorf("unmarshal meta for key %s: %w", k, err)
		}

		if err := fn(k, &m); err != nil {
			return err
		}
	}

	return nil
}
