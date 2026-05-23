package bbolt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

// GroupRepo stores and retrieves auth groups in bbolt.
type GroupRepo struct {
	store *Store
	tx    storage.Tx
}

// NewGroupRepo creates a new GroupRepo backed by the given Store.
func NewGroupRepo(store *Store) *GroupRepo {
	return &GroupRepo{store: store}
}

// WithTx returns a new GroupRepo that uses the provided transaction.
func (r *GroupRepo) WithTx(tx storage.Tx) *GroupRepo {
	return &GroupRepo{
		store: r.store,
		tx:    tx,
	}
}

// Create stores a new group. Returns domain.ErrAlreadyExists if the ID is already taken.
func (r *GroupRepo) Create(_ context.Context, group *domain.Group) error {
	err := r.update(func(tx storage.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))
		key := []byte(group.ID)

		if b.Get(key) != nil {
			return domain.NewAlreadyExistsError("group", group.ID)
		}

		now := time.Now()
		group.CreatedAt = now
		group.UpdatedAt = now
		if group.Version == 0 {
			group.Version = 1
		}

		data, err := json.Marshal(domainToAuthGroupMeta(group))
		if err != nil {
			return fmt.Errorf("marshal group: %w", err)
		}

		return b.Put(key, data)
	})
	if err != nil {
		return fmt.Errorf("create group: %w", err)
	}

	return nil
}

// Get returns the group with the given ID. Returns domain.ErrNotFound if missing.
func (r *GroupRepo) Get(_ context.Context, id string) (*domain.Group, error) {
	var group *domain.Group

	err := r.view(func(tx storage.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))
		data := b.Get([]byte(id))

		if data == nil {
			return domain.NewNotFoundError("group", id)
		}

		m, err := authGroupMetaFromBytes(data)
		if err != nil {
			return err
		}

		group = authGroupMetaToDomain(m)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get group: %w", err)
	}

	return group, nil
}

// Update replaces a group's Name and Members. Returns domain.ErrNotFound if missing.
func (r *GroupRepo) Update(_ context.Context, group *domain.Group) error {
	err := r.update(func(tx storage.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))
		key := []byte(group.ID)
		data := b.Get(key)

		if data == nil {
			return domain.NewNotFoundError("group", group.ID)
		}

		existing, err := authGroupMetaFromBytes(data)
		if err != nil {
			return err
		}

		existing.Name = group.Name
		existing.Description = group.Description
		existing.Version = group.Version
		existing.UpdatedAt = time.Now()

		group.CreatedAt = existing.CreatedAt
		group.UpdatedAt = existing.UpdatedAt
		group.System = existing.System

		newData, err := json.Marshal(existing)
		if err != nil {
			return fmt.Errorf("marshal group: %w", err)
		}

		return b.Put(key, newData)
	})
	if err != nil {
		return fmt.Errorf("update group: %w", err)
	}

	return nil
}

// Delete removes the group with the given ID. Returns domain.ErrNotFound if missing.
func (r *GroupRepo) Delete(_ context.Context, id string) error {
	err := r.update(func(tx storage.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))
		key := []byte(id)

		if b.Get(key) == nil {
			return domain.NewNotFoundError("group", id)
		}

		return b.Delete(key)
	})
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	return nil
}

var errFound = errors.New("found") // sentinel for early ForEach exit

// FindByName returns the first group with the given name.
// Returns domain.ErrNotFound if no group has that name.
func (r *GroupRepo) FindByName(_ context.Context, name string) (*domain.Group, error) {
	var found *domain.Group

	err := r.view(func(tx storage.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))

		return b.ForEach(func(_, v []byte) error {
			m, err := authGroupMetaFromBytes(v)
			if err != nil {
				return err
			}

			if m.Name == name {
				found = authGroupMetaToDomain(m)

				return errFound
			}

			return nil
		})
	})
	if err != nil && !errors.Is(err, errFound) {
		return nil, fmt.Errorf("find group by name: %w", err)
	}

	if found == nil {
		return nil, fmt.Errorf("find group by name: %w", domain.NewNotFoundError("group", name))
	}

	return found, nil
}

// List returns groups matching filter, applies search + sort, and slices the
// result by params.Offset / params.Limit. Total is the count after
// filter+search but before pagination so callers can render page indicators.
//
// bbolt keys groups by UUID, not Name, so name-based filtering requires a
// full bucket scan with per-item check. This is acceptable at the current
// group cardinality (tens, not thousands).
func (r *GroupRepo) List(
	_ context.Context,
	filter domain.GroupFilter,
	params domain.GroupListParams,
) ([]*domain.Group, int, error) {
	var matches []*domain.Group

	err := r.view(func(tx storage.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))

		return b.ForEach(func(_, v []byte) error {
			m, err := authGroupMetaFromBytes(v)
			if err != nil {
				return err
			}

			g := authGroupMetaToDomain(m)

			if !filter.Wildcard {
				if _, ok := filter.Names[g.Name]; !ok {
					return nil
				}
			}

			if !matchesGroupSearch(g.Name, filter.Search) {
				return nil
			}

			matches = append(matches, g)

			return nil
		})
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list groups: %w", err)
	}

	sortGroups(matches, params.Sort)
	total := len(matches)
	paginated := paginateGroups(matches, params.Offset, params.Limit)

	return paginated, total, nil
}

// ListAll returns every group without filter / pagination. Convenience for
// callers that genuinely need the global view; new code that filters by
// caller permissions MUST use List.
func (r *GroupRepo) ListAll(ctx context.Context) ([]*domain.Group, error) {
	groups, _, err := r.List(ctx, domain.GroupFilter{Wildcard: true}, domain.GroupListParams{})
	if err != nil {
		return nil, err
	}

	return groups, nil
}

func matchesGroupSearch(name, search string) bool {
	if search == "" {
		return true
	}

	return strings.Contains(strings.ToLower(name), strings.ToLower(search))
}

func sortGroups(groups []*domain.Group, params domain.SortParams) {
	sort.Slice(groups, func(i, j int) bool {
		a, b := groups[i], groups[j]

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

func paginateGroups(groups []*domain.Group, offset, limit int) []*domain.Group {
	if offset < 0 {
		offset = 0
	}

	if offset >= len(groups) {
		return []*domain.Group{}
	}

	end := len(groups)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}

	return groups[offset:end]
}

func (r *GroupRepo) view(fn func(storage.Tx) error) error {
	if r.tx != nil {
		return fn(r.tx)
	}

	if err := r.store.db.View(func(tx *bolt.Tx) error {
		return fn(&txWrapper{tx: tx})
	}); err != nil {
		return fmt.Errorf("bbolt view: %w", err)
	}

	return nil
}

func (r *GroupRepo) update(fn func(storage.Tx) error) error {
	if r.tx != nil {
		return fn(r.tx)
	}

	if err := r.store.db.Update(func(tx *bolt.Tx) error {
		return fn(&txWrapper{tx: tx})
	}); err != nil {
		return fmt.Errorf("bbolt update: %w", err)
	}

	return nil
}
