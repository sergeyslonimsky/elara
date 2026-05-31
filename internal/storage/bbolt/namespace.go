package bbolt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type NamespaceRepo struct {
	manager *Manager
}

func NewNamespaceRepo(manager *Manager) *NamespaceRepo {
	return &NamespaceRepo{manager: manager}
}

func (r *NamespaceRepo) Create(ctx context.Context, ns *domain.Namespace) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNamespaces))
		// ...
		key := []byte(ns.Name)

		if b.Get(key) != nil {
			return domain.NewAlreadyExistsError("namespace", ns.Name)
		}

		now := time.Now()
		ns.CreatedAt = now
		ns.UpdatedAt = now

		data, err := json.Marshal(domainToNamespaceMeta(ns))
		if err != nil {
			return fmt.Errorf("marshal namespace: %w", err)
		}

		return b.Put(key, data)
	})
	if err != nil {
		return fmt.Errorf("create namespace: %w", err)
	}

	return nil
}

func (r *NamespaceRepo) Get(ctx context.Context, name string) (*domain.Namespace, error) {
	var ns *domain.Namespace

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNamespaces))
		data := b.Get([]byte(name))

		if data == nil {
			return domain.NewNotFoundError("namespace", name)
		}

		var m namespaceMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("unmarshal namespace: %w", err)
		}

		ns = namespaceMetaToDomain(&m, name)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get namespace: %w", err)
	}

	return ns, nil
}

// List returns namespaces matching filter, applies search + sort, and slices
// the result by params.Offset / params.Limit. Total is the count after
// filter+search but before pagination so callers can render page indicators.
//
// When filter.Wildcard is true the repo scans the namespaces bucket; otherwise
// it point-looks up each name in filter.Names. Missing keys in Names are
// silently skipped — they may have been deleted between PDP evaluation and the
// repo call.
func (r *NamespaceRepo) List(
	ctx context.Context,
	filter domain.NamespaceFilter,
	params domain.NamespaceListParams,
) ([]*domain.Namespace, int, error) {
	var matches []*domain.Namespace

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNamespaces))

		if filter.Wildcard {
			return b.ForEach(func(k, v []byte) error {
				ns, err := decodeNamespace(k, v)
				if err != nil {
					return err
				}

				if matchesSearch(ns.Name, filter.Search) {
					matches = append(matches, ns)
				}

				return nil
			})
		}

		for name := range filter.Names {
			data := b.Get([]byte(name))
			if data == nil {
				continue
			}

			ns, err := decodeNamespace([]byte(name), data)
			if err != nil {
				return err
			}

			if matchesSearch(ns.Name, filter.Search) {
				matches = append(matches, ns)
			}
		}

		return nil
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list namespaces: %w", err)
	}

	sortNamespaces(matches, params.Sort)
	total := len(matches)
	paginated := paginate(matches, params.Offset, params.Limit)

	return paginated, total, nil
}

func decodeNamespace(k, v []byte) (*domain.Namespace, error) {
	var m namespaceMeta
	if err := json.Unmarshal(v, &m); err != nil {
		return nil, fmt.Errorf("unmarshal namespace %s: %w", k, err)
	}

	return namespaceMetaToDomain(&m, string(k)), nil
}

func matchesSearch(name, search string) bool {
	if search == "" {
		return true
	}

	return strings.Contains(strings.ToLower(name), strings.ToLower(search))
}

func sortNamespaces(namespaces []*domain.Namespace, params domain.SortParams) {
	sort.Slice(namespaces, func(i, j int) bool {
		a, b := namespaces[i], namespaces[j]

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

func paginate(namespaces []*domain.Namespace, offset, limit int) []*domain.Namespace {
	if offset < 0 {
		offset = 0
	}

	if offset >= len(namespaces) {
		return []*domain.Namespace{}
	}

	end := len(namespaces)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}

	return namespaces[offset:end]
}

// ListAll returns every namespace without filter / pagination. It is a
// convenience for callers that genuinely need the global view (dashboard
// stats, transfer export-all, profile bootstrap) and apply their own scoping
// downstream. New code that filters by caller permissions MUST use List.
func (r *NamespaceRepo) ListAll(ctx context.Context) ([]*domain.Namespace, error) {
	namespaces, _, err := r.List(
		ctx,
		domain.NamespaceFilter{Wildcard: true},
		domain.NamespaceListParams{},
	)
	if err != nil {
		return nil, err
	}

	return namespaces, nil
}

func (r *NamespaceRepo) Update(ctx context.Context, ns *domain.Namespace) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNamespaces))
		data := b.Get([]byte(ns.Name))

		if data == nil {
			return domain.NewNotFoundError("namespace", ns.Name)
		}

		var existing namespaceMeta
		if err := json.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("unmarshal namespace: %w", err)
		}

		if existing.Locked {
			return fmt.Errorf("namespace %q: %w", ns.Name, domain.ErrNamespaceLocked)
		}

		existing.Description = ns.Description
		existing.UpdatedAt = time.Now()

		newData, err := json.Marshal(&existing)
		if err != nil {
			return fmt.Errorf("marshal namespace: %w", err)
		}

		if err := b.Put([]byte(ns.Name), newData); err != nil {
			return fmt.Errorf("put namespace: %w", err)
		}

		ns.CreatedAt = existing.CreatedAt
		ns.UpdatedAt = existing.UpdatedAt

		return nil
	})
	if err != nil {
		return fmt.Errorf("update namespace: %w", err)
	}

	return nil
}

func (r *NamespaceRepo) Delete(ctx context.Context, name string) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNamespaces))
		data := b.Get([]byte(name))

		if data == nil {
			return domain.NewNotFoundError("namespace", name)
		}

		var m namespaceMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("unmarshal namespace: %w", err)
		}

		if m.Locked {
			return fmt.Errorf("namespace %q: %w", name, domain.ErrNamespaceLocked)
		}

		return b.Delete([]byte(name))
	})
	if err != nil {
		return fmt.Errorf("delete namespace: %w", err)
	}

	return nil
}

func (r *NamespaceRepo) CountConfigs(ctx context.Context, name string) (int, error) {
	var count int

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte(bucketMeta))
		prefix := configKeyPrefix(name, "")
		c := meta.Cursor()

		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			count++
		}

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("count namespace configs: %w", err)
	}

	return count, nil
}

func (r *NamespaceRepo) LockNamespace(ctx context.Context, name string) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNamespaces))
		data := b.Get([]byte(name))

		if data == nil {
			return domain.NewNotFoundError("namespace", name)
		}

		var m namespaceMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("unmarshal namespace: %w", err)
		}

		if m.Locked {
			return nil
		}

		m.Locked = true

		newData, err := json.Marshal(&m)
		if err != nil {
			return fmt.Errorf("marshal namespace: %w", err)
		}

		if err := b.Put([]byte(name), newData); err != nil {
			return fmt.Errorf("put namespace: %w", err)
		}

		return writeLockHistory(tx, name, "", domain.EventTypeNamespaceLocked)
	})
	if err != nil {
		return fmt.Errorf("lock namespace: %w", err)
	}

	return nil
}

func (r *NamespaceRepo) UnlockNamespace(ctx context.Context, name string) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNamespaces))
		data := b.Get([]byte(name))

		if data == nil {
			return domain.NewNotFoundError("namespace", name)
		}

		var m namespaceMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("unmarshal namespace: %w", err)
		}

		if !m.Locked {
			return nil
		}

		m.Locked = false

		newData, err := json.Marshal(&m)
		if err != nil {
			return fmt.Errorf("marshal namespace: %w", err)
		}

		if err := b.Put([]byte(name), newData); err != nil {
			return fmt.Errorf("put namespace: %w", err)
		}

		return writeLockHistory(tx, name, "", domain.EventTypeNamespaceUnlocked)
	})
	if err != nil {
		return fmt.Errorf("unlock namespace: %w", err)
	}

	return nil
}

func (r *NamespaceRepo) UpdateTimestamp(ctx context.Context, name string) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNamespaces))
		data := b.Get([]byte(name))

		if data == nil {
			return domain.NewNotFoundError("namespace", name)
		}

		var m namespaceMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("unmarshal namespace: %w", err)
		}

		m.UpdatedAt = time.Now()

		newData, err := json.Marshal(&m)
		if err != nil {
			return fmt.Errorf("marshal namespace: %w", err)
		}

		return b.Put([]byte(name), newData)
	})
	if err != nil {
		return fmt.Errorf("update namespace timestamp: %w", err)
	}

	return nil
}
