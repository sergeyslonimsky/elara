package bbolt_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
)

func TestNamespaceRepo_CRUD(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bbolt.NewNamespaceRepo(bbolt.NewManager(store.DB()))
	ctx := context.Background()

	ns := &domain.Namespace{
		Name:        "production",
		Description: "Production environment",
	}

	// Create
	err := repo.Create(ctx, ns)
	require.NoError(t, err)
	assert.False(t, ns.CreatedAt.IsZero())
	assert.False(t, ns.UpdatedAt.IsZero())

	// Get
	got, err := repo.Get(ctx, "production")
	require.NoError(t, err)
	assert.Equal(t, "production", got.Name)
	assert.Equal(t, "Production environment", got.Description)

	// List
	ns2 := &domain.Namespace{Name: "staging", Description: "Staging"}
	require.NoError(t, repo.Create(ctx, ns2))

	list, total, err := repo.List(
		ctx,
		domain.NamespaceFilter{Wildcard: true},
		domain.NamespaceListParams{},
	)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	assert.Equal(t, 2, total)

	// Delete
	err = repo.Delete(ctx, "staging")
	require.NoError(t, err)

	list, total, err = repo.List(
		ctx,
		domain.NamespaceFilter{Wildcard: true},
		domain.NamespaceListParams{},
	)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, 1, total)

	// Delete non-existent
	err = repo.Delete(ctx, "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestNamespaceRepo_AlreadyExists(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bbolt.NewNamespaceRepo(bbolt.NewManager(store.DB()))
	ctx := context.Background()

	ns := &domain.Namespace{Name: "prod", Description: "Production"}
	require.NoError(t, repo.Create(ctx, ns))

	ns2 := &domain.Namespace{Name: "prod", Description: "Another"}
	err := repo.Create(ctx, ns2)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
}

func TestNamespaceRepo_CountConfigs(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	nsRepo := bbolt.NewNamespaceRepo(bbolt.NewManager(store.DB()))
	cfgRepo := bbolt.NewConfigRepo(bbolt.NewManager(store.DB()))
	ctx := context.Background()

	// Count with no configs
	count, err := nsRepo.CountConfigs(ctx, "prod")
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Add some configs
	for _, path := range []string{"/a.json", "/b.json", "/c.json"} {
		cfg := &domain.Config{
			Path: path, Content: `{}`, Format: domain.FormatJSON, Namespace: "prod",
		}
		require.NoError(t, cfgRepo.Create(ctx, cfg))
	}

	// Config in different namespace
	cfg := &domain.Config{
		Path: "/d.json", Content: `{}`, Format: domain.FormatJSON, Namespace: "staging",
	}
	require.NoError(t, cfgRepo.Create(ctx, cfg))

	count, err = nsRepo.CountConfigs(ctx, "prod")
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	count, err = nsRepo.CountConfigs(ctx, "staging")
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestNamespaceRepo_LockBlocksMutations(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	nsRepo := bbolt.NewNamespaceRepo(bbolt.NewManager(store.DB()))
	cfgRepo := bbolt.NewConfigRepo(bbolt.NewManager(store.DB()))
	ctx := context.Background()

	ns := &domain.Namespace{Name: "prod", Description: "Production"}
	require.NoError(t, nsRepo.Create(ctx, ns))

	cfg := &domain.Config{
		Path: "/a.json", Content: `{}`, Format: domain.FormatJSON, Namespace: "prod",
	}
	require.NoError(t, cfgRepo.Create(ctx, cfg))

	require.NoError(t, nsRepo.LockNamespace(ctx, "prod"))

	// Update description blocked.
	err := nsRepo.Update(ctx, &domain.Namespace{Name: "prod", Description: "new desc"})
	require.ErrorIs(t, err, domain.ErrLocked)
	require.ErrorIs(
		t,
		err,
		domain.ErrNamespaceLocked,
		"namespace-origin lock must satisfy both sentinels",
	)

	// Delete blocked.
	err = nsRepo.Delete(ctx, "prod")
	require.ErrorIs(t, err, domain.ErrLocked)
	require.ErrorIs(t, err, domain.ErrNamespaceLocked)

	// LockConfig blocked inside a locked namespace.
	err = cfgRepo.LockConfig(ctx, "prod", "/a.json")
	require.ErrorIs(t, err, domain.ErrLocked)
	require.ErrorIs(t, err, domain.ErrNamespaceLocked)

	// UnlockConfig blocked inside a locked namespace.
	err = cfgRepo.UnlockConfig(ctx, "prod", "/a.json")
	require.ErrorIs(t, err, domain.ErrLocked)
	require.ErrorIs(t, err, domain.ErrNamespaceLocked)

	// After unlock, namespace mutations work again.
	require.NoError(t, nsRepo.UnlockNamespace(ctx, "prod"))

	err = nsRepo.Update(ctx, &domain.Namespace{Name: "prod", Description: "new desc"})
	require.NoError(t, err)

	require.NoError(t, cfgRepo.LockConfig(ctx, "prod", "/a.json"))
	require.NoError(t, cfgRepo.UnlockConfig(ctx, "prod", "/a.json"))
}

func TestNamespaceRepo_LockWritesHistory(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	nsRepo := bbolt.NewNamespaceRepo(bbolt.NewManager(store.DB()))
	cfgRepo := bbolt.NewConfigRepo(bbolt.NewManager(store.DB()))
	ctx := context.Background()

	require.NoError(t, nsRepo.Create(ctx, &domain.Namespace{Name: "prod"}))
	require.NoError(t, cfgRepo.Create(ctx, &domain.Config{
		Path: "/a.json", Content: `{}`, Format: domain.FormatJSON, Namespace: "prod",
	}))

	require.NoError(t, nsRepo.LockNamespace(ctx, "prod"))
	require.NoError(t, nsRepo.UnlockNamespace(ctx, "prod"))

	// Config history should surface the parent namespace's lock + unlock events.
	entries, err := cfgRepo.GetConfigHistory(ctx, "/a.json", "prod", 20)
	require.NoError(t, err)

	var nsLocked, nsUnlocked int
	for _, e := range entries {
		switch e.EventType {
		case domain.EventTypeNamespaceLocked:
			nsLocked++
		case domain.EventTypeNamespaceUnlocked:
			nsUnlocked++
		case domain.EventTypeCreated, domain.EventTypeUpdated, domain.EventTypeDeleted,
			domain.EventTypeLocked, domain.EventTypeUnlocked:
			// not relevant for this assertion
		}
	}
	assert.Equal(t, 1, nsLocked, "expected a NAMESPACE_LOCKED event in config history")
	assert.Equal(t, 1, nsUnlocked, "expected a NAMESPACE_UNLOCKED event in config history")

	// Activity feed (dashboard) should include both events via the lock changelog.
	recent, err := cfgRepo.ListRecentChanges(ctx, 20)
	require.NoError(t, err)

	var dashLocked, dashUnlocked int
	for _, e := range recent {
		if e.Namespace != "prod" {
			continue
		}
		switch e.Type {
		case domain.EventTypeNamespaceLocked:
			dashLocked++
		case domain.EventTypeNamespaceUnlocked:
			dashUnlocked++
		case domain.EventTypeCreated, domain.EventTypeUpdated, domain.EventTypeDeleted,
			domain.EventTypeLocked, domain.EventTypeUnlocked:
			// not relevant for this assertion
		}
	}
	assert.Equal(t, 1, dashLocked)
	assert.Equal(t, 1, dashUnlocked)
}

func TestConfigRepo_NamespaceLockedPropagates(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	nsRepo := bbolt.NewNamespaceRepo(bbolt.NewManager(store.DB()))
	cfgRepo := bbolt.NewConfigRepo(bbolt.NewManager(store.DB()))
	ctx := context.Background()

	require.NoError(t, nsRepo.Create(ctx, &domain.Namespace{Name: "prod"}))
	require.NoError(t, cfgRepo.Create(ctx, &domain.Config{
		Path: "/a.json", Content: `{}`, Format: domain.FormatJSON, Namespace: "prod",
	}))

	got, err := cfgRepo.Get(ctx, "/a.json", "prod")
	require.NoError(t, err)
	assert.False(t, got.NamespaceLocked)

	require.NoError(t, nsRepo.LockNamespace(ctx, "prod"))

	got, err = cfgRepo.Get(ctx, "/a.json", "prod")
	require.NoError(t, err)
	assert.True(t, got.NamespaceLocked)

	summaries, err := cfgRepo.ListSummariesByPrefix(ctx, "/", "prod")
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.True(t, summaries[0].NamespaceLocked)
}

func TestNamespaceRepo_UpdateTimestamp(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bbolt.NewNamespaceRepo(bbolt.NewManager(store.DB()))
	ctx := context.Background()

	ns := &domain.Namespace{Name: "test", Description: "Test"}
	require.NoError(t, repo.Create(ctx, ns))

	original, err := repo.Get(ctx, "test")
	require.NoError(t, err)

	err = repo.UpdateTimestamp(ctx, "test")
	require.NoError(t, err)

	updated, err := repo.Get(ctx, "test")
	require.NoError(t, err)
	assert.True(
		t,
		updated.UpdatedAt.After(original.UpdatedAt) || updated.UpdatedAt.Equal(original.UpdatedAt),
	)
}

func TestNamespaceRepo_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		seed      []string
		filter    domain.NamespaceFilter
		params    domain.NamespaceListParams
		wantNames []string
		wantTotal int
	}{
		{
			name:      "wildcard filter returns all sorted by name asc",
			seed:      []string{"zeta", "alpha", "beta"},
			filter:    domain.NamespaceFilter{Wildcard: true},
			params:    domain.NamespaceListParams{},
			wantNames: []string{"alpha", "beta", "zeta"},
			wantTotal: 3,
		},
		{
			name: "explicit filter returns subset",
			seed: []string{"ns1", "ns2", "ns3", "ns4", "ns5"},
			filter: domain.NamespaceFilter{
				Names: map[string]struct{}{"ns2": {}, "ns4": {}},
			},
			params:    domain.NamespaceListParams{},
			wantNames: []string{"ns2", "ns4"},
			wantTotal: 2,
		},
		{
			name: "explicit filter with missing name silently skips",
			seed: []string{"exists"},
			filter: domain.NamespaceFilter{
				Names: map[string]struct{}{"exists": {}, "missing": {}},
			},
			params:    domain.NamespaceListParams{},
			wantNames: []string{"exists"},
			wantTotal: 1,
		},
		{
			name:      "search filters by case-insensitive substring",
			seed:      []string{"prod-eu", "PROD-us", "staging", "dev"},
			filter:    domain.NamespaceFilter{Wildcard: true, Search: "PROD"},
			params:    domain.NamespaceListParams{},
			wantNames: []string{"PROD-us", "prod-eu"},
			wantTotal: 2,
		},
		{
			name: "search combined with explicit filter intersects",
			seed: []string{"prod-eu", "prod-us", "staging"},
			filter: domain.NamespaceFilter{
				Names:  map[string]struct{}{"prod-eu": {}, "staging": {}},
				Search: "prod",
			},
			params:    domain.NamespaceListParams{},
			wantNames: []string{"prod-eu"},
			wantTotal: 1,
		},
		{
			name:      "pagination offset+limit slices result, total preserved",
			seed:      []string{"ns1", "ns2", "ns3", "ns4", "ns5"},
			filter:    domain.NamespaceFilter{Wildcard: true},
			params:    domain.NamespaceListParams{Offset: 2, Limit: 2},
			wantNames: []string{"ns3", "ns4"},
			wantTotal: 5,
		},
		{
			name:      "empty filter returns empty list, no error",
			seed:      []string{"ns1", "ns2"},
			filter:    domain.NamespaceFilter{Wildcard: false, Names: nil},
			params:    domain.NamespaceListParams{},
			wantNames: []string{},
			wantTotal: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := newTestStore(t)
			repo := bbolt.NewNamespaceRepo(bbolt.NewManager(store.DB()))
			ctx := t.Context()

			for _, n := range tt.seed {
				require.NoError(t, repo.Create(ctx, &domain.Namespace{Name: n}))
			}

			got, total, err := repo.List(ctx, tt.filter, tt.params)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTotal, total)

			gotNames := make([]string, 0, len(got))
			for _, ns := range got {
				gotNames = append(gotNames, ns.Name)
			}
			assert.Equal(t, tt.wantNames, gotNames)
		})
	}
}

func TestNamespaceRepo_List_SortByModifiedDesc(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bbolt.NewNamespaceRepo(bbolt.NewManager(store.DB()))
	ctx := t.Context()

	require.NoError(t, repo.Create(ctx, &domain.Namespace{Name: "first"}))
	time.Sleep(2 * time.Millisecond)
	require.NoError(t, repo.Create(ctx, &domain.Namespace{Name: "second"}))
	time.Sleep(2 * time.Millisecond)
	require.NoError(t, repo.Create(ctx, &domain.Namespace{Name: "third"}))

	got, total, err := repo.List(ctx,
		domain.NamespaceFilter{Wildcard: true},
		domain.NamespaceListParams{
			Sort: domain.SortParams{Field: "modified", Desc: true},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	require.Len(t, got, 3)
	assert.Equal(t, "third", got[0].Name)
	assert.Equal(t, "second", got[1].Name)
	assert.Equal(t, "first", got[2].Name)
}

func TestNamespaceRepo_ListAll(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bbolt.NewNamespaceRepo(bbolt.NewManager(store.DB()))
	ctx := t.Context()

	for _, n := range []string{"ns-a", "ns-b", "ns-c"} {
		require.NoError(t, repo.Create(ctx, &domain.Namespace{Name: n}))
	}

	got, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, got, 3)

	names := make([]string, 0, len(got))
	for _, ns := range got {
		names = append(names, ns.Name)
	}
	assert.Equal(t, []string{"ns-a", "ns-b", "ns-c"}, names)
}
