package bbolt_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bboltadapter "github.com/sergeyslonimsky/elara/internal/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestNamespaceRepo_CRUD(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewNamespaceRepo(store)
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

	list, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// Delete
	err = repo.Delete(ctx, "staging")
	require.NoError(t, err)

	list, err = repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Delete non-existent
	err = repo.Delete(ctx, "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestNamespaceRepo_AlreadyExists(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewNamespaceRepo(store)
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
	nsRepo := bboltadapter.NewNamespaceRepo(store)
	cfgRepo := bboltadapter.NewConfigRepo(store)
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

func TestNamespaceRepo_UpdateTimestamp(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewNamespaceRepo(store)
	ctx := context.Background()

	ns := &domain.Namespace{Name: "test", Description: "Test"}
	require.NoError(t, repo.Create(ctx, ns))

	original, err := repo.Get(ctx, "test")
	require.NoError(t, err)

	err = repo.UpdateTimestamp(ctx, "test")
	require.NoError(t, err)

	updated, err := repo.Get(ctx, "test")
	require.NoError(t, err)
	assert.True(t, updated.UpdatedAt.After(original.UpdatedAt) || updated.UpdatedAt.Equal(original.UpdatedAt))
}
