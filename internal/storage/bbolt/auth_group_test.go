package bbolt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	bboltadapter "github.com/sergeyslonimsky/elara/internal/storage/bbolt"
)

func TestGroupRepo_Create(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	group := &domain.Group{
		Name:        "admins",
		DisplayName: "Administrators",
	}

	err := repo.Create(ctx, group)
	require.NoError(t, err)
	assert.False(t, group.CreatedAt.IsZero())
	assert.False(t, group.UpdatedAt.IsZero())
}

func TestGroupRepo_Create_Duplicate(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	group := &domain.Group{Name: "ops", DisplayName: "Operations"}
	require.NoError(t, repo.Create(ctx, group))

	err := repo.Create(ctx, &domain.Group{Name: "ops", DisplayName: "Ops Duplicate"})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
}

func TestGroupRepo_Get(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	group := &domain.Group{
		Name:        "devs",
		DisplayName: "Developers",
	}
	require.NoError(t, repo.Create(ctx, group))

	got, err := repo.Get(ctx, "devs")
	require.NoError(t, err)
	assert.Equal(t, "devs", got.Name)
	assert.Equal(t, "Developers", got.DisplayName)
}

func TestGroupRepo_Get_Missing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	_, err := repo.Get(ctx, "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGroupRepo_Update(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	group := &domain.Group{Name: "testers", DisplayName: "Testers"}
	require.NoError(t, repo.Create(ctx, group))

	group.DisplayName = "QA Testers"
	require.NoError(t, repo.Update(ctx, group))

	got, err := repo.Get(ctx, "testers")
	require.NoError(t, err)
	assert.Equal(t, "QA Testers", got.DisplayName)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestGroupRepo_Update_Missing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	err := repo.Update(ctx, &domain.Group{Name: "ghost", DisplayName: "Ghost"})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGroupRepo_Delete(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	group := &domain.Group{Name: "to-delete", DisplayName: "To Delete"}
	require.NoError(t, repo.Create(ctx, group))

	require.NoError(t, repo.Delete(ctx, "to-delete"))

	_, err := repo.Get(ctx, "to-delete")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGroupRepo_Delete_Missing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	err := repo.Delete(ctx, "phantom")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGroupRepo_ListAll(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	// Empty list.
	groups, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, groups)

	// Populate.
	names := []string{"alpha", "beta", "gamma"}
	for _, name := range names {
		g := &domain.Group{Name: name}
		require.NoError(t, repo.Create(ctx, g))
	}

	groups, err = repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Len(t, groups, len(names))
}

// nameSet constructs a domain.GroupFilter.Names map from variadic names.
func nameSet(names ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(names))
	for _, n := range names {
		out[n] = struct{}{}
	}

	return out
}

func groupNames(groups []*domain.Group) []string {
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		out = append(out, g.Name)
	}

	return out
}

func TestGroupRepo_List_FilterSearchPaginateSort(t *testing.T) {
	t.Parallel()

	type seedGroup struct {
		name string
	}

	tests := []struct {
		name      string
		seed      []seedGroup
		filter    domain.GroupFilter
		params    domain.GroupListParams
		wantNames []string
		wantTotal int
	}{
		{
			name: "wildcard returns all sorted by name asc",
			seed: []seedGroup{
				{name: "bbb"},
				{name: "aaa"},
				{name: "ccc"},
			},
			filter:    domain.GroupFilter{Wildcard: true},
			wantNames: []string{"aaa", "bbb", "ccc"},
			wantTotal: 3,
		},
		{
			name: "explicit filter returns subset",
			seed: []seedGroup{
				{name: "a"},
				{name: "b"},
				{name: "c"},
				{name: "d"},
			},
			filter:    domain.GroupFilter{Names: nameSet("a", "c")},
			wantNames: []string{"a", "c"},
			wantTotal: 2,
		},
		{
			name: "explicit filter with missing name silently skips",
			seed: []seedGroup{
				{name: "exists"},
			},
			filter:    domain.GroupFilter{Names: nameSet("exists", "missing")},
			wantNames: []string{"exists"},
			wantTotal: 1,
		},
		{
			name: "search filters case-insensitive substring",
			seed: []seedGroup{
				{name: "dev"},
				{name: "production-dev"},
				{name: "qa"},
				{name: "staging"},
			},
			filter:    domain.GroupFilter{Wildcard: true, Search: "DE"},
			wantNames: []string{"dev", "production-dev"},
			wantTotal: 2,
		},
		{
			name: "search combined with explicit filter intersects",
			seed: []seedGroup{
				{name: "dev"},
				{name: "dev-prod"},
				{name: "qa"},
			},
			filter:    domain.GroupFilter{Names: nameSet("dev", "qa"), Search: "dev"},
			wantNames: []string{"dev"},
			wantTotal: 1,
		},
		{
			name: "pagination offset+limit slices, total preserved",
			seed: []seedGroup{
				{name: "a"},
				{name: "b"},
				{name: "c"},
				{name: "d"},
				{name: "e"},
			},
			filter:    domain.GroupFilter{Wildcard: true},
			params:    domain.GroupListParams{Offset: 2, Limit: 2},
			wantNames: []string{"c", "d"},
			wantTotal: 5,
		},
		{
			name: "empty filter returns empty list",
			seed: []seedGroup{
				{name: "a"},
				{name: "b"},
			},
			filter:    domain.GroupFilter{Wildcard: false, Names: nil},
			wantNames: []string{},
			wantTotal: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := newTestStore(t)
			repo := bboltadapter.NewGroupRepo(bboltadapter.NewManager(store.DB()))
			ctx := t.Context()

			for _, sg := range tt.seed {
				g := &domain.Group{Name: sg.name}
				require.NoError(t, repo.Create(ctx, g))
			}

			got, total, err := repo.List(ctx, tt.filter, tt.params)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTotal, total)
			assert.Equal(t, tt.wantNames, groupNames(got))
		})
	}
}

func TestGroupRepo_SystemAndVersions(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	group := &domain.Group{
		Name:               "sys-admins",
		DisplayName:        "System Admins",
		System:             true,
		MetadataVersion:    42,
		MembersVersion:     7,
		PermissionsVersion: 13,
	}

	// Create — passed counters are persisted as-is.
	require.NoError(t, repo.Create(ctx, group))
	assert.Equal(t, int64(42), group.MetadataVersion)

	// Get and verify
	got, err := repo.Get(ctx, group.Name)
	require.NoError(t, err)
	assert.True(t, got.System)
	assert.Equal(t, int64(42), got.MetadataVersion)
	assert.Equal(t, int64(7), got.MembersVersion)
	assert.Equal(t, int64(13), got.PermissionsVersion)

	// Update (should preserve System flag even if we try to change it)
	got.DisplayName = "Updated Name"
	got.System = false
	got.MetadataVersion = 43
	got.MembersVersion = 8
	got.PermissionsVersion = 14
	require.NoError(t, repo.Update(ctx, got))

	// Get again
	final, err := repo.Get(ctx, group.Name)
	require.NoError(t, err)
	assert.True(t, final.System, "System flag must be preserved from existing record")
	assert.Equal(t, "Updated Name", final.DisplayName)
	assert.Equal(t, int64(43), final.MetadataVersion)
	assert.Equal(t, int64(8), final.MembersVersion)
	assert.Equal(t, int64(14), final.PermissionsVersion)
}
