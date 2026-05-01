package bbolt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bboltadapter "github.com/sergeyslonimsky/elara/internal/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/domain"
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
		ID:      "devs",
		Name:    "Developers",
		Members: []string{"bob@example.com", "carol@example.com"},
	}
	require.NoError(t, repo.Create(ctx, group))

	got, err := repo.Get(ctx, "devs")
	require.NoError(t, err)
	assert.Equal(t, "devs", got.ID)
	assert.Equal(t, "Developers", got.Name)
	assert.Equal(t, []string{"bob@example.com", "carol@example.com"}, got.Members)
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

	group := &domain.Group{ID: "testers", Name: "Testers", Members: []string{"dave@example.com"}}
	require.NoError(t, repo.Create(ctx, group))

	group.Name = "QA Testers"
	group.Members = []string{"dave@example.com", "eve@example.com"}
	require.NoError(t, repo.Update(ctx, group))

	got, err := repo.Get(ctx, "testers")
	require.NoError(t, err)
	assert.Equal(t, "QA Testers", got.Name)
	assert.Equal(t, []string{"dave@example.com", "eve@example.com"}, got.Members)
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

func TestGroupRepo_List(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewGroupRepo(store)
	ctx := t.Context()

	// Empty list.
	groups, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, groups)

	// Populate.
	ids := []string{"alpha", "beta", "gamma"}
	for _, id := range ids {
		g := &domain.Group{ID: id, Name: id}
		require.NoError(t, repo.Create(ctx, g))
	}

	groups, err = repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, groups, len(ids))
}
