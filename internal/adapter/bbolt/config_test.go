package bbolt_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bboltadapter "github.com/sergeyslonimsky/elara/internal/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

func newTestStore(t *testing.T) *bboltadapter.Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")

	store, err := bboltadapter.Open(path)
	require.NoError(t, err)

	t.Cleanup(func() { _ = store.Close() })

	return store
}

func TestConfigRepo_CRUD(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	cfg := &domain.Config{
		Path:      "/services/api/config.json",
		Content:   `{"host": "localhost", "port": 8080}`,
		Format:    domain.FormatJSON,
		Namespace: "default",
		Metadata:  map[string]string{"env": "dev"},
	}

	// Create
	err := repo.Create(ctx, cfg)
	require.NoError(t, err)
	assert.Equal(t, int64(1), cfg.Version)
	assert.Equal(t, int64(1), cfg.Revision)
	assert.NotEmpty(t, cfg.ContentHash)
	assert.False(t, cfg.CreatedAt.IsZero())
	assert.False(t, cfg.UpdatedAt.IsZero())

	// Get
	got, err := repo.Get(ctx, "/services/api/config.json", "default")
	require.NoError(t, err)
	assert.Equal(t, cfg.Path, got.Path)
	assert.Equal(t, cfg.Content, got.Content)
	assert.Equal(t, cfg.ContentHash, got.ContentHash)
	assert.Equal(t, cfg.Format, got.Format)
	assert.Equal(t, int64(1), got.Version)
	assert.Equal(t, int64(1), got.Revision)
	assert.Equal(t, "default", got.Namespace)
	assert.Equal(t, map[string]string{"env": "dev"}, got.Metadata)

	// Update
	cfg.Content = `{"host": "localhost", "port": 9090}`
	err = repo.Update(ctx, cfg)
	require.NoError(t, err)
	assert.Equal(t, int64(2), cfg.Version)
	assert.Equal(t, int64(2), cfg.Revision)

	got, err = repo.Get(ctx, "/services/api/config.json", "default")
	require.NoError(t, err)
	assert.JSONEq(t, `{"host": "localhost", "port": 9090}`, got.Content)
	assert.Equal(t, int64(2), got.Version)

	// Delete
	_, err = repo.Delete(ctx, "/services/api/config.json", "default")
	require.NoError(t, err)

	// Get after delete
	_, err = repo.Get(ctx, "/services/api/config.json", "default")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestConfigRepo_AlreadyExists(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	cfg := &domain.Config{
		Path:      "/test.json",
		Content:   `{}`,
		Format:    domain.FormatJSON,
		Namespace: "default",
	}

	require.NoError(t, repo.Create(ctx, cfg))

	// Second create should fail
	cfg2 := &domain.Config{
		Path:      "/test.json",
		Content:   `{"new": true}`,
		Format:    domain.FormatJSON,
		Namespace: "default",
	}

	err := repo.Create(ctx, cfg2)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
}

func TestConfigRepo_VersionConflict(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	cfg := &domain.Config{
		Path:      "/test.json",
		Content:   `{}`,
		Format:    domain.FormatJSON,
		Namespace: "default",
	}

	require.NoError(t, repo.Create(ctx, cfg))

	// Try update with wrong version
	cfg.Version = 999
	cfg.Content = `{"updated": true}`

	err := repo.Update(ctx, cfg)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConflict)
}

func TestConfigRepo_ListByPrefix(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	configs := []struct {
		path      string
		namespace string
	}{
		{"/services/api/config.json", "prod"},
		{"/services/api/secrets.yaml", "prod"},
		{"/services/web/config.json", "prod"},
		{"/databases/pg.json", "prod"},
		{"/services/api/config.json", "staging"},
	}

	for _, c := range configs {
		cfg := &domain.Config{
			Path: c.path, Content: `{}`, Format: domain.FormatJSON, Namespace: c.namespace,
		}
		require.NoError(t, repo.Create(ctx, cfg))
	}

	// All in prod
	list, err := repo.ListByPrefix(ctx, "/", "prod")
	require.NoError(t, err)
	assert.Len(t, list, 4)

	// Prefix /services in prod
	list, err = repo.ListByPrefix(ctx, "/services", "prod")
	require.NoError(t, err)
	assert.Len(t, list, 3)

	// Prefix /services/api in prod
	list, err = repo.ListByPrefix(ctx, "/services/api", "prod")
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// All in staging
	list, err = repo.ListByPrefix(ctx, "/", "staging")
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestConfigRepo_ListSummaryPage(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	for i := range 5 {
		cfg := &domain.Config{
			Path: "/config" + string(rune('A'+i)) + ".json", Content: `{}`,
			Format: domain.FormatJSON, Namespace: "default",
		}
		require.NoError(t, repo.Create(ctx, cfg))
	}

	// Page 1: limit=2, offset=0
	summaries, total, err := repo.ListSummaryPage(ctx, "/", "default", 2, 0)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, summaries, 2)
	assert.NotEmpty(t, summaries[0].ContentHash)
	assert.NotEmpty(t, summaries[0].Path)

	// Page 2: limit=2, offset=2
	summaries, total, err = repo.ListSummaryPage(ctx, "/", "default", 2, 2)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, summaries, 2)

	// Page 3: limit=2, offset=4
	summaries, total, err = repo.ListSummaryPage(ctx, "/", "default", 2, 4)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, summaries, 1)

	// Beyond total
	summaries, total, err = repo.ListSummaryPage(ctx, "/", "default", 2, 10)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Empty(t, summaries)
}

func TestConfigRepo_History(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	cfg := &domain.Config{
		Path: "/test.json", Content: `{"v": 1}`,
		Format: domain.FormatJSON, Namespace: "default",
	}

	require.NoError(t, repo.Create(ctx, cfg))

	cfg.Content = `{"v": 2}`
	require.NoError(t, repo.Update(ctx, cfg))

	cfg.Content = `{"v": 3}`
	require.NoError(t, repo.Update(ctx, cfg))

	// Get last 2 entries (newest first)
	history, err := repo.GetConfigHistory(ctx, "/test.json", "default", 2)
	require.NoError(t, err)
	assert.Len(t, history, 2)
	assert.Equal(t, `{"v": 3}`, history[0].Content)
	assert.Equal(t, `{"v": 2}`, history[1].Content)
	assert.NotEmpty(t, history[0].ContentHash)
	assert.Equal(t, domain.EventTypeUpdated, history[0].EventType)
	assert.False(t, history[0].Timestamp.IsZero())
	assert.Greater(t, history[0].Revision, history[1].Revision)

	// Get all 3
	history, err = repo.GetConfigHistory(ctx, "/test.json", "default", 10)
	require.NoError(t, err)
	assert.Len(t, history, 3)
	assert.Equal(t, domain.EventTypeCreated, history[2].EventType) // oldest entry has the Created event type
}

func TestConfigRepo_Changelog(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	cfg := &domain.Config{
		Path: "/test.json", Content: `{}`,
		Format: domain.FormatJSON, Namespace: "default",
	}

	require.NoError(t, repo.Create(ctx, cfg))

	cfg.Content = `{"updated": true}`
	require.NoError(t, repo.Update(ctx, cfg))

	_, delErr := repo.Delete(ctx, "/test.json", "default")
	require.NoError(t, delErr)

	// List all changes
	entries, err := repo.ListChanges(ctx, 0, 10)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
	assert.Equal(t, domain.EventTypeCreated, entries[0].Type)
	assert.Equal(t, domain.EventTypeUpdated, entries[1].Type)
	assert.Equal(t, domain.EventTypeDeleted, entries[2].Type)

	// List since revision 1
	entries, err = repo.ListChanges(ctx, 1, 10)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.Equal(t, domain.EventTypeUpdated, entries[0].Type)
}

func TestConfigRepo_SearchByPath(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	configs := []struct {
		path      string
		namespace string
	}{
		{"/services/api/config.json", "prod"},
		{"/services/api/secrets.yaml", "prod"},
		{"/services/web/config.json", "prod"},
		{"/databases/pg.json", "prod"},
		{"/services/api/config.json", "staging"},
	}

	for _, c := range configs {
		cfg := &domain.Config{
			Path: c.path, Content: `{}`, Format: domain.FormatJSON, Namespace: c.namespace,
		}
		require.NoError(t, repo.Create(ctx, cfg))
	}

	// Search "config" across all namespaces (3 matches: prod:api/config, prod:web/config, staging:api/config)
	results, err := repo.SearchByPath(ctx, "config", "")
	require.NoError(t, err)
	assert.Len(t, results, 3)

	// Search "config" in prod only
	results, err = repo.SearchByPath(ctx, "config", "prod")
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Search "secrets"
	results, err = repo.SearchByPath(ctx, "secrets", "")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "/services/api/secrets.yaml", results[0].Path)

	// Case-insensitive
	results, err = repo.SearchByPath(ctx, "CONFIG", "")
	require.NoError(t, err)
	assert.Len(t, results, 3)

	// Search "api" — matches 3 configs (2 in prod, 1 in staging)
	results, err = repo.SearchByPath(ctx, "api", "")
	require.NoError(t, err)
	assert.Len(t, results, 3)

	// No matches
	results, err = repo.SearchByPath(ctx, "nonexistent", "")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestConfigRepo_RevisionMonotonicallyIncreases(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	rev, err := repo.CurrentRevision(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rev)

	cfg := &domain.Config{
		Path: "/a.json", Content: `{}`,
		Format: domain.FormatJSON, Namespace: "default",
	}
	require.NoError(t, repo.Create(ctx, cfg))

	rev, err = repo.CurrentRevision(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rev)

	cfg2 := &domain.Config{
		Path: "/b.json", Content: `{}`,
		Format: domain.FormatJSON, Namespace: "default",
	}
	require.NoError(t, repo.Create(ctx, cfg2))

	rev, err = repo.CurrentRevision(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), rev)

	cfg.Content = `{"changed": true}`
	require.NoError(t, repo.Update(ctx, cfg))

	rev, err = repo.CurrentRevision(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), rev)
}
