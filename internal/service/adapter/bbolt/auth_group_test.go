package bbolt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	bboltadapter "github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
)

func TestGroupRepo_Create(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(store)
	ctx := t.Context()

	group := &domain.Group{
		ID:      "admins",
		Name:    "Administrators",
		Members: []string{"alice@example.com"},
	}

	err := repo.Create(ctx, group)
	require.NoError(t, err)
	assert.False(t, group.CreatedAt.IsZero())
	assert.False(t, group.UpdatedAt.IsZero())
}

func TestGroupRepo_Create_Duplicate(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(store)
	ctx := t.Context()

	group := &domain.Group{ID: "ops", Name: "Operations"}
	require.NoError(t, repo.Create(ctx, group))

	err := repo.Create(ctx, &domain.Group{ID: "ops", Name: "Ops Duplicate"})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
}

func TestGroupRepo_Get(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(store)
	ctx := t.Context()

	group := &domain.Group{
		ID:   "devs",
		Name: "Developers",
	}
	require.NoError(t, repo.Create(ctx, group))

	got, err := repo.Get(ctx, "devs")
	require.NoError(t, err)
	assert.Equal(t, "devs", got.ID)
	assert.Equal(t, "Developers", got.Name)
}

func TestGroupRepo_Get_Missing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(store)
	ctx := t.Context()

	_, err := repo.Get(ctx, "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGroupRepo_Update(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(store)
	ctx := t.Context()

	group := &domain.Group{ID: "testers", Name: "Testers"}
	require.NoError(t, repo.Create(ctx, group))

	group.Name = "QA Testers"
	require.NoError(t, repo.Update(ctx, group))

	got, err := repo.Get(ctx, "testers")
	require.NoError(t, err)
	assert.Equal(t, "QA Testers", got.Name)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestGroupRepo_Update_Missing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(store)
	ctx := t.Context()

	err := repo.Update(ctx, &domain.Group{ID: "ghost", Name: "Ghost"})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGroupRepo_Delete(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(store)
	ctx := t.Context()

	group := &domain.Group{ID: "to-delete", Name: "To Delete"}
	require.NoError(t, repo.Create(ctx, group))

	require.NoError(t, repo.Delete(ctx, "to-delete"))

	_, err := repo.Get(ctx, "to-delete")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGroupRepo_Delete_Missing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(store)
	ctx := t.Context()

	err := repo.Delete(ctx, "phantom")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGroupRepo_FindByName(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(store)
	ctx := t.Context()

	group := &domain.Group{ID: "find-me", Name: "FindMe"}
	require.NoError(t, repo.Create(ctx, group))

	got, err := repo.FindByName(ctx, "FindMe")
	require.NoError(t, err)
	assert.Equal(t, "find-me", got.ID)
	assert.Equal(t, "FindMe", got.Name)
}

func TestGroupRepo_FindByName_Missing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(store)
	ctx := t.Context()

	_, err := repo.FindByName(ctx, "nonexistent-name")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGroupRepo_FindByName_Multiple(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(store)
	ctx := t.Context()

	require.NoError(t, repo.Create(ctx, &domain.Group{ID: "first", Name: "First Group"}))
	require.NoError(t, repo.Create(ctx, &domain.Group{ID: "second", Name: "Second Group"}))

	got, err := repo.FindByName(ctx, "Second Group")
	require.NoError(t, err)
	assert.Equal(t, "second", got.ID)
	assert.Equal(t, "Second Group", got.Name)
}

func TestGroupRepo_ListAll(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(store)
	ctx := t.Context()

	// Empty list.
	groups, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, groups)

	// Populate.
	ids := []string{"alpha", "beta", "gamma"}
	for _, id := range ids {
		g := &domain.Group{ID: id, Name: id}
		require.NoError(t, repo.Create(ctx, g))
	}

	groups, err = repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Len(t, groups, len(ids))
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
		id      string
		name    string
		members []string
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
				{id: "id-bbb", name: "bbb"},
				{id: "id-aaa", name: "aaa"},
				{id: "id-ccc", name: "ccc"},
			},
			filter:    domain.GroupFilter{Wildcard: true},
			wantNames: []string{"aaa", "bbb", "ccc"},
			wantTotal: 3,
		},
		{
			name: "explicit filter returns subset",
			seed: []seedGroup{
				{id: "id-a", name: "a"},
				{id: "id-b", name: "b"},
				{id: "id-c", name: "c"},
				{id: "id-d", name: "d"},
			},
			filter:    domain.GroupFilter{Names: nameSet("a", "c")},
			wantNames: []string{"a", "c"},
			wantTotal: 2,
		},
		{
			name: "explicit filter with missing name silently skips",
			seed: []seedGroup{
				{id: "id-exists", name: "exists"},
			},
			filter:    domain.GroupFilter{Names: nameSet("exists", "missing")},
			wantNames: []string{"exists"},
			wantTotal: 1,
		},
		{
			name: "search filters case-insensitive substring",
			seed: []seedGroup{
				{id: "id-dev", name: "dev"},
				{id: "id-pdev", name: "production-dev"},
				{id: "id-qa", name: "qa"},
				{id: "id-staging", name: "staging"},
			},
			filter:    domain.GroupFilter{Wildcard: true, Search: "DE"},
			wantNames: []string{"dev", "production-dev"},
			wantTotal: 2,
		},
		{
			name: "search combined with explicit filter intersects",
			seed: []seedGroup{
				{id: "id-dev", name: "dev"},
				{id: "id-dev2", name: "dev-prod"},
				{id: "id-qa", name: "qa"},
			},
			filter:    domain.GroupFilter{Names: nameSet("dev", "qa"), Search: "dev"},
			wantNames: []string{"dev"},
			wantTotal: 1,
		},
		{
			name: "pagination offset+limit slices, total preserved",
			seed: []seedGroup{
				{id: "id-a", name: "a"},
				{id: "id-b", name: "b"},
				{id: "id-c", name: "c"},
				{id: "id-d", name: "d"},
				{id: "id-e", name: "e"},
			},
			filter:    domain.GroupFilter{Wildcard: true},
			params:    domain.GroupListParams{Offset: 2, Limit: 2},
			wantNames: []string{"c", "d"},
			wantTotal: 5,
		},
		{
			name: "empty filter returns empty list",
			seed: []seedGroup{
				{id: "id-a", name: "a"},
				{id: "id-b", name: "b"},
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
			repo := bboltadapter.NewGroupRepo(store)
			ctx := t.Context()

			for _, sg := range tt.seed {
				g := &domain.Group{ID: sg.id, Name: sg.name, Members: sg.members}
				require.NoError(t, repo.Create(ctx, g))
			}

			got, total, err := repo.List(ctx, tt.filter, tt.params)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTotal, total)
			assert.Equal(t, tt.wantNames, groupNames(got))
		})
	}
}

func TestGroupRepo_SystemAndVersion(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(store)
	ctx := t.Context()

	group := &domain.Group{
		ID:      "sys-admins",
		Name:    "System Admins",
		System:  true,
		Version: 42,
	}

	// Create
	require.NoError(t, repo.Create(ctx, group))
	assert.Equal(t, int64(42), group.Version)

	// Get and verify
	got, err := repo.Get(ctx, group.ID)
	require.NoError(t, err)
	assert.True(t, got.System)
	assert.Equal(t, int64(42), got.Version)

	// Update (should preserve System flag even if we try to change it)
	got.Name = "Updated Name"
	got.System = false
	got.Version = 43
	require.NoError(t, repo.Update(ctx, got))

	// Get again
	final, err := repo.Get(ctx, group.ID)
	require.NoError(t, err)
	assert.True(t, final.System, "System flag must be preserved from existing record")
	assert.Equal(t, "Updated Name", final.Name)
	assert.Equal(t, int64(43), final.Version)
}
