package group_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	grouprepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/group"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

func newRepo(t *testing.T) (*grouprepo.Repository, pkgbbolt.Manager) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "elara.db")
	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	mgr := pkgbbolt.NewManager(store.DB())

	return grouprepo.NewRepository(mgr), mgr
}

func newGroup(t *testing.T, overrides ...func(*domain.Group)) *domain.Group {
	t.Helper()

	g := &domain.Group{
		Name:        "devs",
		DisplayName: "Developers",
		Description: "Development team",
	}
	for _, o := range overrides {
		o(g)
	}

	return g
}

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

func TestRepository_Create(t *testing.T) {
	t.Parallel()

	t.Run("success sets timestamps and metadata version", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		g := newGroup(t)
		require.NoError(t, repo.Create(ctx, g))

		assert.False(t, g.CreatedAt.IsZero())
		assert.False(t, g.UpdatedAt.IsZero())
		assert.Equal(t, int64(1), g.MetadataVersion)

		got, err := repo.Get(ctx, g.Name)
		require.NoError(t, err)
		assert.Equal(t, g.Name, got.Name)
		assert.Equal(t, g.DisplayName, got.DisplayName)
		assert.Equal(t, g.Description, got.Description)
	})

	t.Run("preserves caller-supplied MetadataVersion when non-zero", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		g := newGroup(t, func(g *domain.Group) { g.MetadataVersion = 42 })
		require.NoError(t, repo.Create(ctx, g))

		got, err := repo.Get(ctx, g.Name)
		require.NoError(t, err)
		assert.Equal(t, int64(42), got.MetadataVersion)
	})

	t.Run("duplicate name returns ErrAlreadyExists", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		first := newGroup(t, func(g *domain.Group) { g.Name = "ops" })
		require.NoError(t, repo.Create(ctx, first))

		err := repo.Create(ctx, newGroup(t, func(g *domain.Group) { g.Name = "ops" }))
		require.ErrorIs(t, err, storage.ErrResourceAlreadyExists)
	})
}

func TestRepository_Get(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		g := newGroup(t)
		require.NoError(t, repo.Create(ctx, g))

		got, err := repo.Get(ctx, g.Name)
		require.NoError(t, err)
		assert.Equal(t, g.Name, got.Name)
		assert.Equal(t, g.DisplayName, got.DisplayName)
	})

	t.Run("missing returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		_, err := repo.Get(t.Context(), "nope")
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_Update(t *testing.T) {
	t.Parallel()

	t.Run("persists mutable fields and refreshes UpdatedAt", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		g := newGroup(t)
		require.NoError(t, repo.Create(ctx, g))
		createdAt := g.CreatedAt

		// Sleep a tiny bit so UpdatedAt strictly advances (millisecond resolution should suffice).
		time.Sleep(2 * time.Millisecond)

		g.DisplayName = "QA Devs"
		g.Description = "Quality assurance"
		g.MetadataVersion = 5
		g.MembersVersion = 9
		g.PermissionsVersion = 11
		require.NoError(t, repo.Update(ctx, g))

		got, err := repo.Get(ctx, g.Name)
		require.NoError(t, err)
		assert.Equal(t, "QA Devs", got.DisplayName)
		assert.Equal(t, "Quality assurance", got.Description)
		assert.Equal(t, int64(5), got.MetadataVersion)
		assert.Equal(t, int64(9), got.MembersVersion)
		assert.Equal(t, int64(11), got.PermissionsVersion)
		assert.True(t, got.CreatedAt.Equal(createdAt), "CreatedAt must be preserved")
		assert.True(t, got.UpdatedAt.After(createdAt), "UpdatedAt must be refreshed")
	})

	t.Run("missing returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		err := repo.Update(t.Context(), &domain.Group{Name: "ghost"})
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})

	t.Run("preserves System flag from persisted record", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		g := newGroup(t, func(g *domain.Group) {
			g.Name = "sys-admins"
			g.System = true
		})
		require.NoError(t, repo.Create(ctx, g))

		// Try to clear System via update — must be ignored.
		g.System = false
		g.DisplayName = "Renamed"
		require.NoError(t, repo.Update(ctx, g))

		got, err := repo.Get(ctx, g.Name)
		require.NoError(t, err)
		assert.True(t, got.System, "System flag must be preserved from existing record")
		assert.Equal(t, "Renamed", got.DisplayName)
	})
}

func TestRepository_Delete(t *testing.T) {
	t.Parallel()

	t.Run("success removes record", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		g := newGroup(t, func(g *domain.Group) { g.Name = "to-delete" })
		require.NoError(t, repo.Create(ctx, g))

		require.NoError(t, repo.Delete(ctx, g.Name))

		_, err := repo.Get(ctx, g.Name)
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})

	t.Run("missing returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		err := repo.Delete(t.Context(), "phantom")
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		seedNames []string
		filter    domain.GroupFilter
		params    domain.GroupListParams
		wantNames []string
		wantTotal int
	}{
		{
			name:      "wildcard returns all sorted by name asc",
			seedNames: []string{"bbb", "aaa", "ccc"},
			filter:    domain.GroupFilter{Wildcard: true},
			wantNames: []string{"aaa", "bbb", "ccc"},
			wantTotal: 3,
		},
		{
			name:      "explicit filter returns subset",
			seedNames: []string{"a", "b", "c", "d"},
			filter:    domain.GroupFilter{Names: nameSet("a", "c")},
			wantNames: []string{"a", "c"},
			wantTotal: 2,
		},
		{
			name:      "explicit filter with missing name silently skips",
			seedNames: []string{"exists"},
			filter:    domain.GroupFilter{Names: nameSet("exists", "missing")},
			wantNames: []string{"exists"},
			wantTotal: 1,
		},
		{
			name:      "search filters case-insensitive substring",
			seedNames: []string{"dev", "production-dev", "qa", "staging"},
			filter:    domain.GroupFilter{Wildcard: true, Search: "DE"},
			wantNames: []string{"dev", "production-dev"},
			wantTotal: 2,
		},
		{
			name:      "search combined with explicit filter intersects",
			seedNames: []string{"dev", "dev-prod", "qa"},
			filter:    domain.GroupFilter{Names: nameSet("dev", "qa"), Search: "dev"},
			wantNames: []string{"dev"},
			wantTotal: 1,
		},
		{
			name:      "pagination offset+limit slices, total preserved",
			seedNames: []string{"a", "b", "c", "d", "e"},
			filter:    domain.GroupFilter{Wildcard: true},
			params:    domain.GroupListParams{Offset: 2, Limit: 2},
			wantNames: []string{"c", "d"},
			wantTotal: 5,
		},
		{
			name:      "offset beyond length returns empty slice but full total",
			seedNames: []string{"a", "b"},
			filter:    domain.GroupFilter{Wildcard: true},
			params:    domain.GroupListParams{Offset: 99, Limit: 10},
			wantNames: []string{},
			wantTotal: 2,
		},
		{
			name:      "empty filter (no wildcard, no names) returns empty list",
			seedNames: []string{"a", "b"},
			filter:    domain.GroupFilter{},
			wantNames: []string{},
			wantTotal: 0,
		},
		{
			name:      "empty bucket returns empty",
			seedNames: nil,
			filter:    domain.GroupFilter{Wildcard: true},
			wantNames: []string{},
			wantTotal: 0,
		},
		{
			name:      "sort by name desc",
			seedNames: []string{"a", "b", "c"},
			filter:    domain.GroupFilter{Wildcard: true},
			params:    domain.GroupListParams{Sort: domain.SortParams{Field: "name", Desc: true}},
			wantNames: []string{"c", "b", "a"},
			wantTotal: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, _ := newRepo(t)
			ctx := t.Context()

			for _, n := range tt.seedNames {
				require.NoError(t, repo.Create(ctx, &domain.Group{Name: n}))
			}

			got, total, err := repo.List(ctx, tt.filter, tt.params)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTotal, total)
			assert.Equal(t, tt.wantNames, groupNames(got))
		})
	}
}

func TestRepository_ListAll(t *testing.T) {
	t.Parallel()

	t.Run("empty repo returns empty slice", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		got, err := repo.ListAll(t.Context())
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("returns every group", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		for _, n := range []string{"alpha", "beta", "gamma"} {
			require.NoError(t, repo.Create(ctx, &domain.Group{Name: n}))
		}

		got, err := repo.ListAll(ctx)
		require.NoError(t, err)
		assert.Len(t, got, 3)
	})
}

func TestRepository_SystemAndVersionsRoundTrip(t *testing.T) {
	t.Parallel()

	repo, _ := newRepo(t)
	ctx := t.Context()

	g := &domain.Group{
		Name:               "sys-admins",
		DisplayName:        "System Admins",
		System:             true,
		MetadataVersion:    42,
		MembersVersion:     7,
		PermissionsVersion: 13,
	}
	require.NoError(t, repo.Create(ctx, g))

	got, err := repo.Get(ctx, g.Name)
	require.NoError(t, err)
	assert.True(t, got.System)
	assert.Equal(t, int64(42), got.MetadataVersion)
	assert.Equal(t, int64(7), got.MembersVersion)
	assert.Equal(t, int64(13), got.PermissionsVersion)
}

func TestRepository_WithTx_RollbackDiscardsWrites(t *testing.T) {
	t.Parallel()

	repo, mgr := newRepo(t)
	ctx := t.Context()

	err := mgr.WithTx(ctx, func(ctx context.Context) error {
		if err := repo.Create(ctx, &domain.Group{Name: "a"}); err != nil {
			return err
		}
		if err := repo.Create(ctx, &domain.Group{Name: "b"}); err != nil {
			return err
		}

		return assert.AnError
	})
	require.ErrorIs(t, err, assert.AnError)

	got, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, got)
}
