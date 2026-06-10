// Package config is the bbolt-backed repository for domain.Config.
//
// Repository contract:
//   - Public methods preserve the legacy ConfigRepo signatures so callers in
//     internal/usecase and internal/handler/etcdv3 are unchanged.
//   - Multi-bucket writes (Create / Update / Delete / Lock / Unlock) reconcile
//     the content, meta, history, changelog, lock_history, and lock_changelog
//     buckets inside a single WithTx so reads never observe a half-applied
//     state.
//   - Errors are storage-level (storage.ErrResourceNotFound /
//     storage.ErrResourceAlreadyExists) except for the domain-meaningful
//     errors that remain as domain values (domain.ErrLocked,
//     domain.ErrNamespaceLocked, domain.NewConflictError for version
//     conflicts). Usecase callers translate storage sentinels to domain
//     sentinels at the boundary.
//   - The repo performs a cross-bucket read on the foreign `namespaces`
//     bucket (isNamespaceLocked) — this is a same-tx read, not a call into
//     the namespace repository.
package config

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/storage/internal"
	"github.com/sergeyslonimsky/elara/pkg/bbolt"
)

const (
	bucketContent       = "content"
	bucketMeta          = "meta"
	bucketNamespaces    = "namespaces"
	bucketChangelog     = "changelog"
	bucketHistory       = "history"
	bucketSys           = "sys"
	bucketLockHistory   = "lock_history"
	bucketLockChangelog = "lock_changelog"

	sysRevisionKeyName = "revision"
	sysLockSeqKeyName  = "lock_event_seq"
)

// Repository stores and retrieves configs in bbolt.
type Repository struct {
	dbm bbolt.Manager
}

// NewRepository creates a new Repository backed by the given Manager.
func NewRepository(dbm bbolt.Manager) *Repository {
	return &Repository{dbm: dbm}
}

// Create persists a new config and atomically writes the content, meta,
// history, and changelog buckets. Returns storage.ErrResourceAlreadyExists if
// the (namespace, path) key is already present, and domain.ErrNamespaceLocked
// if the parent namespace is locked.
func (r *Repository) Create(ctx context.Context, cfg *domain.Config) error {
	err := r.dbm.WithTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)

		if err := validateNamespaceUnlocked(q, cfg.Namespace); err != nil {
			return err
		}

		key := configKey(cfg.Namespace, cfg.Path)
		if bbolt.Exists(q, bucketMeta, key) {
			return fmt.Errorf("config %s: %w", cfg.Path, storage.ErrResourceAlreadyExists)
		}

		cfg.GenerateHash()
		cfg.SetDefaults()

		now := time.Now()
		cfg.Version = 1
		cfg.CreatedAt = now
		cfg.UpdatedAt = now

		revision, err := nextRevision(q)
		if err != nil {
			return err
		}

		cfg.Revision = revision
		cfg.CreateRevision = revision

		return writeConfigEntry(q, key, cfg, revision, domain.EventTypeCreated)
	})
	if err != nil {
		return fmt.Errorf("create config: %w", err)
	}

	return nil
}

// Get retrieves a config by path and namespace.
// Returns storage.ErrResourceNotFound if no such config exists.
func (r *Repository) Get(ctx context.Context, path, namespace string) (*domain.Config, error) {
	var cfg *domain.Config

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)
		key := configKey(namespace, path)

		m, err := bbolt.Get[internal.ConfigMeta](q, bucketMeta, key)
		if errors.Is(err, bbolt.ErrNotFound) {
			return fmt.Errorf("config %s: %w", path, storage.ErrResourceNotFound)
		}
		if err != nil {
			return fmt.Errorf("get meta: %w", err)
		}

		content := q.Bucket(bucketContent).Get(key)
		cfg = internal.ConfigMetaToDomain(m, string(content), path, namespace)

		nsLocked, err := isNamespaceLocked(q, namespace)
		if err != nil {
			return err
		}

		cfg.NamespaceLocked = nsLocked

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}

	return cfg, nil
}

// Update writes the config with optimistic locking on Version. Returns
// storage.ErrResourceNotFound if the config does not exist; domain.ErrLocked
// if the config itself is locked; domain.NewConflictError on version
// mismatch; domain.ErrNamespaceLocked if the parent namespace is locked.
func (r *Repository) Update(ctx context.Context, cfg *domain.Config) error {
	err := r.dbm.WithTx(ctx, func(ctx context.Context) error {
		return updateConfigTx(r.dbm.GetQuerier(ctx), cfg)
	})
	if err != nil {
		return fmt.Errorf("update config: %w", err)
	}

	return nil
}

// Delete removes a config and writes a deletion changelog entry. Returns
// storage.ErrResourceNotFound if the config does not exist; domain.ErrLocked
// if the config is locked; domain.ErrNamespaceLocked if the parent namespace
// is locked. The returned int64 is the new global revision.
func (r *Repository) Delete(ctx context.Context, path, namespace string) (int64, error) {
	var newRev int64

	err := r.dbm.WithTx(ctx, func(ctx context.Context) error {
		rev, err := deleteConfigTx(r.dbm.GetQuerier(ctx), path, namespace)
		newRev = rev

		return err
	})
	if err != nil {
		return 0, fmt.Errorf("delete config: %w", err)
	}

	return newRev, nil
}

// ListByPrefix returns all configs whose key matches (namespace, pathPrefix).
func (r *Repository) ListByPrefix(
	ctx context.Context,
	pathPrefix, namespace string,
) ([]*domain.Config, error) {
	var configs []*domain.Config

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)
		content := q.Bucket(bucketContent)
		nsLocks := nsLockCache{}

		return scanMeta(q, namespace, pathPrefix, func(key []byte, m internal.ConfigMeta) error {
			ns, path := parseConfigKey(key)
			contentBytes := content.Get(key)
			cfg := internal.ConfigMetaToDomain(m, string(contentBytes), path, ns)

			nsLocked, err := nsLocks.get(q, ns)
			if err != nil {
				return err
			}

			cfg.NamespaceLocked = nsLocked
			configs = append(configs, cfg)

			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("list configs by prefix: %w", err)
	}

	return configs, nil
}

// ListAllByNamespace returns every config in the given namespace.
func (r *Repository) ListAllByNamespace(
	ctx context.Context,
	namespace string,
) ([]*domain.Config, error) {
	return r.ListByPrefix(ctx, "", namespace)
}

// ListSummariesByPrefix returns summaries (without content) for all configs
// whose key matches (namespace, pathPrefix).
func (r *Repository) ListSummariesByPrefix(
	ctx context.Context,
	pathPrefix, namespace string,
) ([]*domain.ConfigSummary, error) {
	var summaries []*domain.ConfigSummary

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)
		nsLocks := nsLockCache{}

		return scanMeta(q, namespace, pathPrefix, func(key []byte, m internal.ConfigMeta) error {
			ns, path := parseConfigKey(key)
			s := internal.ConfigMetaToSummary(m, path, ns)

			nsLocked, err := nsLocks.get(q, ns)
			if err != nil {
				return err
			}

			s.NamespaceLocked = nsLocked
			summaries = append(summaries, s)

			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("list config summaries by prefix: %w", err)
	}

	return summaries, nil
}

// ListSummaryPage returns a paginated page of config summaries.
func (r *Repository) ListSummaryPage(
	ctx context.Context,
	pathPrefix, namespace string,
	limit, offset int,
) ([]*domain.ConfigSummary, int, error) {
	var (
		summaries []*domain.ConfigSummary
		total     int
	)

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)
		nsLocks := nsLockCache{}
		idx := 0

		return scanMeta(q, namespace, pathPrefix, func(key []byte, m internal.ConfigMeta) error {
			if idx >= offset && len(summaries) < limit {
				ns, path := parseConfigKey(key)
				s := internal.ConfigMetaToSummary(m, path, ns)

				nsLocked, err := nsLocks.get(q, ns)
				if err != nil {
					return err
				}

				s.NamespaceLocked = nsLocked
				summaries = append(summaries, s)
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
func (r *Repository) ListConfigPage(
	ctx context.Context,
	pathPrefix, namespace string,
	limit, offset int,
) ([]*domain.Config, int, error) {
	var (
		configs []*domain.Config
		total   int
	)

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)
		content := q.Bucket(bucketContent)
		nsLocks := nsLockCache{}
		idx := 0

		return scanMeta(q, namespace, pathPrefix, func(key []byte, m internal.ConfigMeta) error {
			if idx >= offset && len(configs) < limit {
				ns, path := parseConfigKey(key)
				contentBytes := content.Get(key)
				cfg := internal.ConfigMetaToDomain(m, string(contentBytes), path, ns)

				nsLocked, err := nsLocks.get(q, ns)
				if err != nil {
					return err
				}

				cfg.NamespaceLocked = nsLocked
				configs = append(configs, cfg)
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
func (r *Repository) CountByNamespace(ctx context.Context, namespace string) (int, error) {
	var count int

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		return scanMeta(r.dbm.GetQuerier(ctx), namespace, "/", func(_ []byte, _ internal.ConfigMeta) error {
			count++

			return nil
		})
	})
	if err != nil {
		return 0, fmt.Errorf("count configs by namespace: %w", err)
	}

	return count, nil
}

// SearchByPath returns all configs whose path contains query (case-insensitive).
// Sorting and pagination is done in the usecase layer.
func (r *Repository) SearchByPath(
	ctx context.Context,
	query, namespace string,
) ([]*domain.ConfigSummary, error) {
	var results []*domain.ConfigSummary

	queryLower := strings.ToLower(query)

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		meta := r.dbm.GetQuerier(ctx).Bucket(bucketMeta)
		codec := bbolt.JSONCodec[internal.ConfigMeta]{}
		c := meta.Cursor()

		var (
			k, v   []byte
			prefix []byte
		)

		if namespace != "" {
			prefix = configKeyPrefix(namespace, "")
			k, v = c.Seek(prefix)
		} else {
			k, v = c.First()
		}

		for ; k != nil; k, v = c.Next() {
			if namespace != "" && !bytes.HasPrefix(k, prefix) {
				break
			}

			ns, path := parseConfigKey(k)
			if !strings.Contains(strings.ToLower(path), queryLower) {
				continue
			}

			var m internal.ConfigMeta
			if err := codec.Unmarshal(v, &m); err != nil {
				return fmt.Errorf("unmarshal meta: %w", err)
			}

			results = append(results, internal.ConfigMetaToSummary(m, path, ns))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("search configs by path: %w", err)
	}

	return results, nil
}

// GetConfigHistory returns the last `limit` history entries for a config,
// newest first. Merges content history (history + changelog buckets) with
// the config's own lock events and its namespace's lock events.
func (r *Repository) GetConfigHistory(
	ctx context.Context,
	path, namespace string,
	limit int,
) ([]*domain.HistoryEntry, error) {
	var entries []*domain.HistoryEntry

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)
		prefix := historyPrefix(namespace, path)

		contentEntries := collectContentHistory(q, prefix)

		lockEntries := collectConfigLockHistory(q, prefix)
		lockEntries = append(lockEntries, collectNamespaceLockHistory(q, namespace)...)

		entries = mergeHistoryEntries(contentEntries, lockEntries, limit)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get config history: %w", err)
	}

	return entries, nil
}

// GetAtRevision returns the history entry at the given revision (or the
// closest earlier one for the same config). Returns
// storage.ErrResourceNotFound if no entry is found.
func (r *Repository) GetAtRevision(
	ctx context.Context,
	path, namespace string,
	revision int64,
) (*domain.HistoryEntry, error) {
	var entry *domain.HistoryEntry

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)
		history := q.Bucket(bucketHistory)
		changelog := q.Bucket(bucketChangelog)
		seekKey := historyKey(namespace, path, revision)
		prefix := historyPrefix(namespace, path)

		c := history.Cursor()
		k, v := c.Seek(seekKey)

		if k == nil || !bytes.HasPrefix(k, prefix) {
			k, v = c.Prev()
		} else if !bytes.Equal(k, seekKey) {
			k, v = c.Prev()
		}

		if k == nil || !bytes.HasPrefix(k, prefix) {
			return fmt.Errorf("config history %s: %w", path, storage.ErrResourceNotFound)
		}

		rev := parseRevision(k[len(prefix):])

		entry = &domain.HistoryEntry{
			Revision:    rev,
			Content:     string(v),
			ContentHash: computeHash(v),
		}

		if clData := changelog.Get(revisionBytes(rev)); clData != nil {
			var cl internal.ChangelogEntry
			if err := (bbolt.JSONCodec[internal.ChangelogEntry]{}).Unmarshal(clData, &cl); err == nil {
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

// CurrentRevision returns the current global revision counter.
func (r *Repository) CurrentRevision(ctx context.Context) (int64, error) {
	var rev int64

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		b := r.dbm.GetQuerier(ctx).Bucket(bucketSys).Get([]byte(sysRevisionKeyName))
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

// ListChanges returns changelog entries strictly after sinceRevision, up to limit.
func (r *Repository) ListChanges(
	ctx context.Context,
	sinceRevision int64,
	limit int,
) ([]*domain.ChangelogEntry, error) {
	var entries []*domain.ChangelogEntry

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		changelog := r.dbm.GetQuerier(ctx).Bucket(bucketChangelog)
		codec := bbolt.JSONCodec[internal.ChangelogEntry]{}
		seekKey := revisionBytes(sinceRevision + 1)

		c := changelog.Cursor()
		for k, v := c.Seek(seekKey); k != nil && len(entries) < limit; k, v = c.Next() {
			var e internal.ChangelogEntry
			if err := codec.Unmarshal(v, &e); err != nil {
				return fmt.Errorf("unmarshal changelog entry: %w", err)
			}

			rev := parseRevision(k)
			entries = append(entries, internal.ChangelogEntryToDomain(e, rev))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list changes: %w", err)
	}

	return entries, nil
}

// ListRecentChanges returns the most recent changelog entries (newest first)
// merged from the content changelog and the lock changelog buckets.
func (r *Repository) ListRecentChanges(
	ctx context.Context,
	limit int,
) ([]*domain.ChangelogEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	var entries []*domain.ChangelogEntry

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)
		contentEntries := collectRecentChangelog(q.Bucket(bucketChangelog), limit)
		lockEntries := collectRecentChangelog(q.Bucket(bucketLockChangelog), limit)

		all := make([]*domain.ChangelogEntry, 0, len(contentEntries)+len(lockEntries))
		all = append(all, contentEntries...)
		all = append(all, lockEntries...)

		sort.Slice(all, func(i, j int) bool {
			return all[i].Timestamp.After(all[j].Timestamp)
		})

		if len(all) > limit {
			all = all[:limit]
		}

		entries = all

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list recent changes: %w", err)
	}

	return entries, nil
}

// LockConfig marks a config as locked. Idempotent. Returns
// storage.ErrResourceNotFound if missing; domain.ErrNamespaceLocked if the
// parent namespace is locked.
func (r *Repository) LockConfig(ctx context.Context, namespace, path string) error {
	if err := r.toggleConfigLock(ctx, namespace, path, true); err != nil {
		return fmt.Errorf("lock config: %w", err)
	}

	return nil
}

// UnlockConfig mirrors LockConfig.
func (r *Repository) UnlockConfig(ctx context.Context, namespace, path string) error {
	if err := r.toggleConfigLock(ctx, namespace, path, false); err != nil {
		return fmt.Errorf("unlock config: %w", err)
	}

	return nil
}

func (r *Repository) toggleConfigLock(
	ctx context.Context,
	namespace, path string,
	locked bool,
) error {
	err := r.dbm.WithTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)

		if err := validateNamespaceUnlocked(q, namespace); err != nil {
			return err
		}

		key := configKey(namespace, path)

		m, err := bbolt.Get[internal.ConfigMeta](q, bucketMeta, key)
		if errors.Is(err, bbolt.ErrNotFound) {
			return fmt.Errorf("config %s: %w", path, storage.ErrResourceNotFound)
		}
		if err != nil {
			return fmt.Errorf("get meta: %w", err)
		}

		if m.Locked == locked {
			return nil
		}

		m.Locked = locked
		if err := bbolt.Put(q, bucketMeta, key, m); err != nil {
			return fmt.Errorf("put meta: %w", err)
		}

		eventType := domain.EventTypeUnlocked
		if locked {
			eventType = domain.EventTypeLocked
		}

		return writeLockHistory(q, namespace, path, eventType)
	})
	if err != nil {
		return fmt.Errorf("wrap tx: %w", err)
	}

	return nil
}

// updateConfigTx is the inner body of Update — it expects to run inside a
// write transaction (q must be tx-backed).
func updateConfigTx(q bbolt.Querier, cfg *domain.Config) error {
	if err := validateNamespaceUnlocked(q, cfg.Namespace); err != nil {
		return err
	}

	key := configKey(cfg.Namespace, cfg.Path)

	existing, err := bbolt.Get[internal.ConfigMeta](q, bucketMeta, key)
	if errors.Is(err, bbolt.ErrNotFound) {
		return fmt.Errorf("config %s: %w", cfg.Path, storage.ErrResourceNotFound)
	}
	if err != nil {
		return fmt.Errorf("get existing meta: %w", err)
	}

	if err := validateUpdatePreconditions(existing, cfg); err != nil {
		return err
	}

	cfg.GenerateHash()

	now := time.Now()
	cfg.Version = existing.Version + 1
	cfg.CreatedAt = existing.CreatedAt
	cfg.UpdatedAt = now
	cfg.CreateRevision = existing.CreateRevision

	revision, err := nextRevision(q)
	if err != nil {
		return err
	}

	cfg.Revision = revision

	return writeConfigEntry(q, key, cfg, revision, domain.EventTypeUpdated)
}

// deleteConfigTx is the inner body of Delete.
func deleteConfigTx(q bbolt.Querier, path, namespace string) (int64, error) {
	if err := validateNamespaceUnlocked(q, namespace); err != nil {
		return 0, err
	}

	key := configKey(namespace, path)

	existing, err := bbolt.Get[internal.ConfigMeta](q, bucketMeta, key)
	if errors.Is(err, bbolt.ErrNotFound) {
		return 0, fmt.Errorf("config %s: %w", path, storage.ErrResourceNotFound)
	}
	if err != nil {
		return 0, fmt.Errorf("get existing meta: %w", err)
	}

	if existing.Locked {
		return 0, fmt.Errorf("config %q: %w", path, domain.ErrLocked)
	}

	revision, err := nextRevision(q)
	if err != nil {
		return 0, err
	}

	if err := bbolt.Delete(q, bucketContent, key); err != nil {
		return 0, fmt.Errorf("delete content: %w", err)
	}

	if err := bbolt.Delete(q, bucketMeta, key); err != nil {
		return 0, fmt.Errorf("delete meta: %w", err)
	}

	if err := writeChangelog(q, revision, domain.EventTypeDeleted, path, namespace, 0); err != nil {
		return 0, err
	}

	return revision, nil
}

// writeConfigEntry persists content, meta, history, and changelog for one
// config write in a single tx.
func writeConfigEntry(
	q bbolt.Querier,
	key []byte,
	cfg *domain.Config,
	revision int64,
	eventType domain.EventType,
) error {
	if err := q.Bucket(bucketContent).Put(key, []byte(cfg.Content)); err != nil {
		return fmt.Errorf("put content: %w", err)
	}

	if err := bbolt.Put(q, bucketMeta, key, internal.DomainToConfigMeta(cfg)); err != nil {
		return fmt.Errorf("put meta: %w", err)
	}

	if err := writeHistory(q, cfg.Namespace, cfg.Path, revision, []byte(cfg.Content)); err != nil {
		return err
	}

	return writeChangelog(q, revision, eventType, cfg.Path, cfg.Namespace, cfg.Version)
}

func writeHistory(q bbolt.Querier, namespace, path string, revision int64, content []byte) error {
	if err := q.Bucket(bucketHistory).Put(historyKey(namespace, path, revision), content); err != nil {
		return fmt.Errorf("put history: %w", err)
	}

	return nil
}

func writeChangelog(
	q bbolt.Querier,
	revision int64,
	eventType domain.EventType,
	path, namespace string,
	version int64,
) error {
	entry := internal.ChangelogEntry{
		Type:      int(eventType),
		Path:      path,
		Namespace: namespace,
		Version:   version,
		Timestamp: time.Now(),
	}

	if err := bbolt.Put(q, bucketChangelog, revisionBytes(revision), entry); err != nil {
		return fmt.Errorf("put changelog: %w", err)
	}

	return nil
}

func writeLockHistory(q bbolt.Querier, namespace, path string, eventType domain.EventType) error {
	seq, err := nextLockSeq(q)
	if err != nil {
		return err
	}

	now := time.Now()

	entry := internal.LockHistoryEntry{
		Type:      int(eventType),
		Timestamp: now,
	}
	histKey := append(historyPrefix(namespace, path), revisionBytes(seq)...)
	if err := bbolt.Put(q, bucketLockHistory, histKey, entry); err != nil {
		return fmt.Errorf("put lock history: %w", err)
	}

	cl := internal.ChangelogEntry{
		Type:      int(eventType),
		Path:      path,
		Namespace: namespace,
		Timestamp: now,
	}
	if err := bbolt.Put(q, bucketLockChangelog, revisionBytes(seq), cl); err != nil {
		return fmt.Errorf("put lock changelog: %w", err)
	}

	return nil
}

func nextRevision(q bbolt.Querier) (int64, error) {
	sys := q.Bucket(bucketSys)
	current := parseRevision(sys.Get([]byte(sysRevisionKeyName)))
	next := current + 1

	if err := sys.Put([]byte(sysRevisionKeyName), revisionBytes(next)); err != nil {
		return 0, fmt.Errorf("update revision: %w", err)
	}

	return next, nil
}

func nextLockSeq(q bbolt.Querier) (int64, error) {
	sys := q.Bucket(bucketSys)
	current := parseRevision(sys.Get([]byte(sysLockSeqKeyName)))
	next := current + 1

	if err := sys.Put([]byte(sysLockSeqKeyName), revisionBytes(next)); err != nil {
		return 0, fmt.Errorf("update lock seq: %w", err)
	}

	return next, nil
}

func validateUpdatePreconditions(existing internal.ConfigMeta, cfg *domain.Config) error {
	if existing.Locked {
		return fmt.Errorf("update precondition: %w", domain.NewLockedError(cfg.Path))
	}

	if existing.Version != cfg.Version {
		return fmt.Errorf(
			"update precondition: %w",
			domain.NewConflictError(cfg.Version, existing.Version),
		)
	}

	return nil
}

// nsLockCache caches namespace-lock lookups within a single transaction.
type nsLockCache map[string]bool

func (c nsLockCache) get(q bbolt.Querier, namespace string) (bool, error) {
	if v, ok := c[namespace]; ok {
		return v, nil
	}

	v, err := isNamespaceLocked(q, namespace)
	if err != nil {
		return false, err
	}

	c[namespace] = v

	return v, nil
}

// isNamespaceLocked reads the foreign `namespaces` bucket inside the current
// tx. This is a cross-bucket read intentionally kept in the config package;
// see the package doc comment.
func isNamespaceLocked(q bbolt.Querier, namespace string) (bool, error) {
	data := q.Bucket(bucketNamespaces).Get([]byte(namespace))
	if data == nil {
		return false, nil
	}

	var m internal.NamespaceMeta
	if err := (bbolt.JSONCodec[internal.NamespaceMeta]{}).Unmarshal(data, &m); err != nil {
		return false, fmt.Errorf("unmarshal namespace meta: %w", err)
	}

	return m.Locked, nil
}

func validateNamespaceUnlocked(q bbolt.Querier, namespace string) error {
	locked, err := isNamespaceLocked(q, namespace)
	if err != nil {
		return err
	}

	if locked {
		return fmt.Errorf("namespace %q: %w", namespace, domain.ErrNamespaceLocked)
	}

	return nil
}

func collectContentHistory(q bbolt.Querier, prefix []byte) []*domain.HistoryEntry {
	history := q.Bucket(bucketHistory)
	changelog := q.Bucket(bucketChangelog)
	codec := bbolt.JSONCodec[internal.ChangelogEntry]{}

	var entries []*domain.HistoryEntry

	c := history.Cursor()
	for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
		keyCopy := make([]byte, len(k))
		copy(keyCopy, k)

		content := history.Get(keyCopy)
		rev := parseRevision(keyCopy[len(prefix):])

		entry := &domain.HistoryEntry{
			Revision:    rev,
			Content:     string(content),
			ContentHash: computeHash(content),
		}

		if clData := changelog.Get(revisionBytes(rev)); clData != nil {
			var cl internal.ChangelogEntry
			if err := codec.Unmarshal(clData, &cl); err == nil {
				entry.EventType = domain.EventType(cl.Type)
				entry.Timestamp = cl.Timestamp
			}
		}

		entries = append(entries, entry)
	}

	return entries
}

func collectConfigLockHistory(q bbolt.Querier, prefix []byte) []*domain.HistoryEntry {
	lockHistory := q.Bucket(bucketLockHistory)
	codec := bbolt.JSONCodec[internal.LockHistoryEntry]{}

	var entries []*domain.HistoryEntry

	c := lockHistory.Cursor()
	for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
		var lhe internal.LockHistoryEntry
		if err := codec.Unmarshal(v, &lhe); err != nil {
			continue
		}

		entries = append(entries, &domain.HistoryEntry{
			EventType: domain.EventType(lhe.Type),
			Timestamp: lhe.Timestamp,
		})
	}

	return entries
}

// collectNamespaceLockHistory reads the namespace-level lock events stored
// under "<ns>\x00\x00<seq>" so a config's history surfaces its parent
// namespace's lock transitions.
func collectNamespaceLockHistory(q bbolt.Querier, namespace string) []*domain.HistoryEntry {
	lockHistory := q.Bucket(bucketLockHistory)
	codec := bbolt.JSONCodec[internal.LockHistoryEntry]{}
	nsPrefix := historyPrefix(namespace, "")

	var entries []*domain.HistoryEntry

	c := lockHistory.Cursor()
	for k, v := c.Seek(nsPrefix); k != nil && bytes.HasPrefix(k, nsPrefix); k, v = c.Next() {
		if len(k)-len(nsPrefix) != revisionSize {
			continue
		}

		var lhe internal.LockHistoryEntry
		if err := codec.Unmarshal(v, &lhe); err != nil {
			continue
		}

		entries = append(entries, &domain.HistoryEntry{
			EventType: domain.EventType(lhe.Type),
			Timestamp: lhe.Timestamp,
		})
	}

	return entries
}

func collectRecentChangelog(bkt bbolt.Bucket, limit int) []*domain.ChangelogEntry {
	codec := bbolt.JSONCodec[internal.ChangelogEntry]{}

	var entries []*domain.ChangelogEntry

	c := bkt.Cursor()
	for k, v := c.Last(); k != nil && len(entries) < limit; k, v = c.Prev() {
		var e internal.ChangelogEntry
		if err := codec.Unmarshal(v, &e); err != nil {
			continue
		}

		rev := parseRevision(k)
		entries = append(entries, internal.ChangelogEntryToDomain(e, rev))
	}

	return entries
}

// scanMeta iterates the `meta` bucket optionally filtered by namespace +
// path prefix. When namespace is empty, iterates all keys but skips entries
// that don't match a cross-namespace pathPrefix filter.
func scanMeta(
	q bbolt.Querier,
	namespace, pathPrefix string,
	fn func(key []byte, m internal.ConfigMeta) error,
) error {
	meta := q.Bucket(bucketMeta)
	codec := bbolt.JSONCodec[internal.ConfigMeta]{}
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

		var m internal.ConfigMeta
		if err := codec.Unmarshal(v, &m); err != nil {
			return fmt.Errorf("unmarshal meta for key %s: %w", k, err)
		}

		if err := fn(k, m); err != nil {
			return err
		}
	}

	return nil
}

// shouldSkipByPath returns true when a key should be skipped because it
// does not match a cross-namespace path-prefix filter.
func shouldSkipByPath(k []byte, namespace, pathPrefix string) bool {
	if namespace != "" || pathPrefix == "" || pathPrefix == "/" {
		return false
	}

	_, path := parseConfigKey(k)

	return !strings.HasPrefix(path, pathPrefix)
}

func mergeHistoryEntries(content, lock []*domain.HistoryEntry, limit int) []*domain.HistoryEntry {
	merged := make([]*domain.HistoryEntry, 0, len(content)+len(lock))
	merged = append(merged, content...)
	merged = append(merged, lock...)

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Timestamp.After(merged[j].Timestamp)
	})

	if len(merged) > limit {
		merged = merged[:limit]
	}

	return merged
}

func computeHash(data []byte) string {
	h := sha256.Sum256(data)

	return hex.EncodeToString(h[:])
}
