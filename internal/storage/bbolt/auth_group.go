package bbolt

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

var _ domain.GroupReader = (*GroupRepo)(nil)

// GroupRepo stores and retrieves auth groups in bbolt.
type GroupRepo struct {
	manager *Manager
}

// NewGroupRepo creates a new GroupRepo backed by the given Manager.
func NewGroupRepo(manager *Manager) *GroupRepo {
	return &GroupRepo{manager: manager}
}

// Create stores a new group. Returns domain.ErrAlreadyExists if the name is already taken.
func (r *GroupRepo) Create(ctx context.Context, group *domain.Group) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))
		key := []byte(group.Name)

		if b.Get(key) != nil {
			return domain.NewAlreadyExistsError("group", group.Name)
		}

		now := time.Now()
		group.CreatedAt = now
		group.UpdatedAt = now
		if group.MetadataVersion == 0 {
			group.MetadataVersion = 1
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

// Get returns the group with the given name. Returns domain.ErrNotFound if missing.
func (r *GroupRepo) Get(ctx context.Context, name string) (*domain.Group, error) {
	var group *domain.Group

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))
		data := b.Get([]byte(name))

		if data == nil {
			return domain.NewNotFoundError("group", name)
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

// Update replaces a group's metadata. Returns domain.ErrNotFound if missing.
func (r *GroupRepo) Update(ctx context.Context, group *domain.Group) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))
		key := []byte(group.Name)
		data := b.Get(key)

		if data == nil {
			return domain.NewNotFoundError("group", group.Name)
		}

		existing, err := authGroupMetaFromBytes(data)
		if err != nil {
			return err
		}

		existing.DisplayName = group.DisplayName
		existing.Description = group.Description
		existing.MetadataVersion = group.MetadataVersion
		existing.MembersVersion = group.MembersVersion
		existing.PermissionsVersion = group.PermissionsVersion
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

// Delete removes the group with the given name. Returns domain.ErrNotFound if missing.
func (r *GroupRepo) Delete(ctx context.Context, name string) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))
		key := []byte(name)

		if b.Get(key) == nil {
			return domain.NewNotFoundError("group", name)
		}

		return b.Delete(key)
	})
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	return nil
}

// List returns groups matching filter, applies search + sort, and slices the
// result by params.Offset / params.Limit. Total is the count after
// filter+search but before pagination so callers can render page indicators.
//
// bbolt keys groups by Name, so name-based filtering could in principle use
// point lookups. We keep a single ForEach scan because the search filter still
// requires touching every record and group cardinality is low.
func (r *GroupRepo) List(
	ctx context.Context,
	filter domain.GroupFilter,
	params domain.GroupListParams,
) ([]*domain.Group, int, error) {
	var matches []*domain.Group

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
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
