// Package group is the bbolt-backed repository for domain.Group
// (RBAC group entities).
//
// Repository contract:
//   - Pure CRUD (Create, Get, Update, Delete) is "dumb": the caller
//     prepares every mutable field, the repo writes verbatim. The only
//     server-side mutations are CreatedAt/UpdatedAt timestamps and the
//     initial MetadataVersion bump on Create.
//   - Update is a read-modify-write: it preserves CreatedAt and the
//     System flag from the persisted record (those are not caller-
//     editable) and refreshes UpdatedAt.
//   - Membership and permissions live exclusively in Casbin (g-rules
//     / p-rules); this repo never touches them.
//   - Errors are storage-level (storage.ErrResourceNotFound,
//     storage.ErrResourceAlreadyExists). Usecase callers translate
//     them to domain errors at the boundary.
package group

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

var _ domain.GroupReader = (*Repository)(nil)

// Repository stores and retrieves auth groups in bbolt.
type Repository struct {
	dbm bbolt.Manager
}

// NewRepository creates a new Repository backed by the given Manager.
func NewRepository(dbm bbolt.Manager) *Repository {
	return &Repository{dbm: dbm}
}

// Create stores a new group. Returns storage.ErrResourceAlreadyExists if the
// name is already taken.
//
// Atomicity (exists-check + put) is the caller's responsibility — wrap in
// Manager.WithTx when invoking concurrently with other writers.
func (r *Repository) Create(ctx context.Context, group *domain.Group) error {
	q := r.dbm.GetQuerier(ctx)
	key := []byte(group.Name)

	if bbolt.Exists(q, bucketGroups, key) {
		return fmt.Errorf("group %s: %w", group.Name, storage.ErrResourceAlreadyExists)
	}

	now := time.Now()
	group.CreatedAt = now
	group.UpdatedAt = now
	if group.MetadataVersion == 0 {
		group.MetadataVersion = 1
	}

	if err := bbolt.Put(q, bucketGroups, key, internal.DomainToGroupMeta(group)); err != nil {
		return fmt.Errorf("create group: %w", err)
	}

	return nil
}

// Get returns the group with the given name. Returns storage.ErrResourceNotFound if missing.
func (r *Repository) Get(ctx context.Context, name string) (*domain.Group, error) {
	m, err := bbolt.Get[internal.GroupMeta](r.dbm.GetQuerier(ctx), bucketGroups, []byte(name))
	if errors.Is(err, bbolt.ErrNotFound) {
		return nil, fmt.Errorf("group %s: %w", name, storage.ErrResourceNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get group: %w", err)
	}

	return internal.GroupMetaToDomain(m), nil
}

// Update replaces a group's metadata. Returns storage.ErrResourceNotFound if missing.
//
// Atomicity of the read-modify-write is the caller's responsibility — wrap
// in Manager.WithTx when concurrent writers may touch the same group.
func (r *Repository) Update(ctx context.Context, group *domain.Group) error {
	q := r.dbm.GetQuerier(ctx)
	key := []byte(group.Name)

	existing, err := bbolt.Get[internal.GroupMeta](q, bucketGroups, key)
	if errors.Is(err, bbolt.ErrNotFound) {
		return fmt.Errorf("group %s: %w", group.Name, storage.ErrResourceNotFound)
	}
	if err != nil {
		return fmt.Errorf("update group: %w", err)
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

	if err := bbolt.Put(q, bucketGroups, key, existing); err != nil {
		return fmt.Errorf("update group: %w", err)
	}

	return nil
}

// Delete removes the group with the given name. Returns storage.ErrResourceNotFound if missing.
//
// Atomicity (exists-check + delete) is the caller's responsibility.
func (r *Repository) Delete(ctx context.Context, name string) error {
	q := r.dbm.GetQuerier(ctx)
	key := []byte(name)

	if !bbolt.Exists(q, bucketGroups, key) {
		return fmt.Errorf("group %s: %w", name, storage.ErrResourceNotFound)
	}

	if err := bbolt.Delete(q, bucketGroups, key); err != nil {
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
func (r *Repository) List(
	ctx context.Context,
	filter domain.GroupFilter,
	params domain.GroupListParams,
) ([]*domain.Group, int, error) {
	var matches []*domain.Group

	bucket := r.dbm.GetQuerier(ctx).Bucket(bucketGroups)
	codec := bbolt.JSONCodec[internal.GroupMeta]{}

	err := bucket.ForEach(func(_, v []byte) error {
		var m internal.GroupMeta
		if err := codec.Unmarshal(v, &m); err != nil {
			return fmt.Errorf("decode group: %w", err)
		}

		g := internal.GroupMetaToDomain(m)

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
func (r *Repository) ListAll(ctx context.Context) ([]*domain.Group, error) {
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
