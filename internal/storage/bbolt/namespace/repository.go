// Package namespace is the bbolt-backed repository for domain.Namespace.
//
// Repository contract:
//   - Pure CRUD (Create, Get, Update, Delete) is "dumb" — callers prepare
//     all fields, repository writes verbatim.
//   - List applies search/sort/pagination — that is data shaping, not
//     business validation.
//   - Dedicated mutators (UpdateTimestamp, LockNamespace, UnlockNamespace)
//     own their own timestamp / sequence logic and write atomically across
//     the namespaces, lock_history, lock_changelog, and sys buckets.
//   - Counting configs per namespace lives in the config repository
//     (config.Repository.CountByNamespace) — the namespace repo does not
//     read foreign buckets.
package namespace

import (
	"context"
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
	bucketName          = "namespaces"
	bucketLockHistory   = "lock_history"
	bucketLockChangelog = "lock_changelog"
	bucketSys           = "sys"
	sysLockSeqKey       = "lock_event_seq"
)

type Repository struct {
	dbm bbolt.Manager
}

func NewRepository(dbm bbolt.Manager) *Repository {
	return &Repository{dbm: dbm}
}

// Create persists a brand-new namespace. Caller MUST set CreatedAt/UpdatedAt
// and any other fields — the repo writes verbatim.
func (r *Repository) Create(ctx context.Context, ns *domain.Namespace) error {
	q := r.dbm.GetQuerier(ctx)

	if bbolt.Exists(q, bucketName, []byte(ns.Name)) {
		return fmt.Errorf("namespace %s exists: %w", ns.Name, storage.ErrResourceAlreadyExists)
	}

	if err := bbolt.Put(q, bucketName, []byte(ns.Name), internal.DomainToNamespaceMeta(ns)); err != nil {
		return fmt.Errorf("put: %w", err)
	}

	return nil
}

func (r *Repository) Get(ctx context.Context, name string) (*domain.Namespace, error) {
	m, err := bbolt.Get[internal.NamespaceMeta](r.dbm.GetQuerier(ctx), bucketName, []byte(name))
	if errors.Is(err, bbolt.ErrNotFound) {
		return nil, fmt.Errorf("namespace %s: %w", name, storage.ErrResourceNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get: %w", err)
	}

	return internal.NamespaceMetaToDomain(m, name), nil
}

// Update writes the namespace whole-record. Callers are responsible for
// read-modify-write of fields they want to preserve and for setting
// UpdatedAt. Lock-state changes go through LockNamespace / UnlockNamespace
// (so the audit history is written atomically).
func (r *Repository) Update(ctx context.Context, ns *domain.Namespace) error {
	q := r.dbm.GetQuerier(ctx)

	if !bbolt.Exists(q, bucketName, []byte(ns.Name)) {
		return fmt.Errorf("namespace %s: %w", ns.Name, storage.ErrResourceNotFound)
	}

	if err := bbolt.Put(q, bucketName, []byte(ns.Name), internal.DomainToNamespaceMeta(ns)); err != nil {
		return fmt.Errorf("put: %w", err)
	}

	return nil
}

func (r *Repository) Delete(ctx context.Context, name string) error {
	q := r.dbm.GetQuerier(ctx)

	if !bbolt.Exists(q, bucketName, []byte(name)) {
		return fmt.Errorf("namespace %s: %w", name, storage.ErrResourceNotFound)
	}

	if err := bbolt.Delete(q, bucketName, []byte(name)); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	return nil
}

// List returns namespaces matching filter, applies search + sort, and slices
// the result by params.Offset / params.Limit. Total is the count after
// filter+search but before pagination so callers can render page indicators.
//
// When filter.Wildcard is true the repo scans the namespaces bucket; otherwise
// it point-looks up each name in filter.Names. Missing keys in Names are
// silently skipped — they may have been deleted between PDP evaluation and the
// repo call.
func (r *Repository) List(
	ctx context.Context,
	filter domain.NamespaceFilter,
	params domain.NamespaceListParams,
) ([]*domain.Namespace, int, error) {
	var matches []*domain.Namespace

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		bucket := r.dbm.GetQuerier(ctx).Bucket(bucketName)
		codec := bbolt.JSONCodec[internal.NamespaceMeta]{}

		if filter.Wildcard {
			return bucket.ForEach(func(k, v []byte) error {
				var m internal.NamespaceMeta
				if err := codec.Unmarshal(v, &m); err != nil {
					return fmt.Errorf("decode %s: %w", k, err)
				}

				ns := internal.NamespaceMetaToDomain(m, string(k))
				if matchesSearch(ns.Name, filter.Search) {
					matches = append(matches, ns)
				}

				return nil
			})
		}

		for name := range filter.Names {
			data := bucket.Get([]byte(name))
			if data == nil {
				continue
			}

			var m internal.NamespaceMeta
			if err := codec.Unmarshal(data, &m); err != nil {
				return fmt.Errorf("decode %s: %w", name, err)
			}

			ns := internal.NamespaceMetaToDomain(m, name)
			if matchesSearch(ns.Name, filter.Search) {
				matches = append(matches, ns)
			}
		}

		return nil
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list: %w", err)
	}

	sortNamespaces(matches, params.Sort)
	total := len(matches)
	paginated := paginate(matches, params.Offset, params.Limit)

	return paginated, total, nil
}

// ListAll returns every namespace without filter / pagination. Convenience
// for callers that genuinely need the global view (dashboard stats, transfer
// export-all, profile bootstrap) and apply their own scoping downstream.
// New code that filters by caller permissions MUST use List.
func (r *Repository) ListAll(ctx context.Context) ([]*domain.Namespace, error) {
	out, _, err := r.List(
		ctx,
		domain.NamespaceFilter{Wildcard: true},
		domain.NamespaceListParams{},
	)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// UpdateTimestamp bumps UpdatedAt to time.Now(). Dedicated mutator used by
// config writes so they can refresh the namespace recency without doing a
// full read-modify-write at the service layer.
func (r *Repository) UpdateTimestamp(ctx context.Context, name string) error {
	err := r.dbm.WithTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)

		m, err := bbolt.Get[internal.NamespaceMeta](q, bucketName, []byte(name))
		if errors.Is(err, bbolt.ErrNotFound) {
			return fmt.Errorf("namespace %s: %w", name, storage.ErrResourceNotFound)
		}
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}

		m.UpdatedAt = time.Now()

		return bbolt.Put(q, bucketName, []byte(name), m)
	})
	if err != nil {
		return fmt.Errorf("update timestamp: %w", err)
	}

	return nil
}

// LockNamespace flips Locked → true and atomically writes audit entries to
// lock_history and lock_changelog. No-op (returns nil) if the namespace is
// already locked. Returns ErrResourceNotFound if it does not exist.
func (r *Repository) LockNamespace(ctx context.Context, name string) error {
	return r.toggleLock(ctx, name, true, domain.EventTypeNamespaceLocked)
}

// UnlockNamespace mirrors LockNamespace.
func (r *Repository) UnlockNamespace(ctx context.Context, name string) error {
	return r.toggleLock(ctx, name, false, domain.EventTypeNamespaceUnlocked)
}

func (r *Repository) toggleLock(
	ctx context.Context,
	name string,
	locked bool,
	eventType domain.EventType,
) error {
	verb := "unlock"
	if locked {
		verb = "lock"
	}

	q := r.dbm.GetQuerier(ctx)

	m, err := bbolt.Get[internal.NamespaceMeta](q, bucketName, []byte(name))
	if errors.Is(err, bbolt.ErrNotFound) {
		return fmt.Errorf("%s namespace %s: %w", verb, name, storage.ErrResourceNotFound)
	}
	if err != nil {
		return fmt.Errorf("%s namespace: %w", verb, err)
	}

	if m.Locked == locked {
		return nil
	}

	m.Locked = locked
	if err := bbolt.Put(q, bucketName, []byte(name), m); err != nil {
		return fmt.Errorf("%s namespace: %w", verb, err)
	}

	if err := writeLockEvent(q, name, "", eventType); err != nil {
		return fmt.Errorf("%s namespace: %w", verb, err)
	}

	return nil
}

// writeLockEvent appends one lock event to lock_history + lock_changelog and
// bumps the sys.lock_event_seq counter. Must run inside a write tx that
// already holds the namespace mutation — the caller's WithTx provides it.
func writeLockEvent(
	q bbolt.Querier,
	namespace, path string,
	eventType domain.EventType,
) error {
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

func nextLockSeq(q bbolt.Querier) (int64, error) {
	sys := q.Bucket(bucketSys)
	current := parseRevision(sys.Get([]byte(sysLockSeqKey)))
	next := current + 1

	if err := sys.Put([]byte(sysLockSeqKey), revisionBytes(next)); err != nil {
		return 0, fmt.Errorf("update lock seq: %w", err)
	}

	return next, nil
}

func matchesSearch(name, search string) bool {
	if search == "" {
		return true
	}

	return strings.Contains(strings.ToLower(name), strings.ToLower(search))
}

func sortNamespaces(out []*domain.Namespace, params domain.SortParams) {
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]

		var less bool

		switch params.Field {
		case "modified":
			less = a.UpdatedAt.Before(b.UpdatedAt)
		default:
			less = a.Name < b.Name
		}

		if params.Desc {
			return !less
		}

		return less
	})
}

func paginate(out []*domain.Namespace, offset, limit int) []*domain.Namespace {
	if offset < 0 {
		offset = 0
	}

	if offset >= len(out) {
		return []*domain.Namespace{}
	}

	end := len(out)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}

	return out[offset:end]
}
