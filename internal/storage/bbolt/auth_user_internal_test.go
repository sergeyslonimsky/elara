package bbolt

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(path)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = store.Close()
	})

	return store
}

func TestUserRepo_Delete(t *testing.T) {
	t.Parallel()

	t.Run("Delete existing user", func(t *testing.T) {
		t.Parallel()

		store := newTestStore(t)
		repo := NewUserRepo(NewManager(store.db))
		ctx := t.Context()

		user := &domain.User{
			Email: "test@example.com",
			Name:  "Test User",
		}

		err := repo.Upsert(ctx, user)
		require.NoError(t, err)

		err = repo.Delete(ctx, user.Email)
		require.NoError(t, err)

		got, err := repo.Get(ctx, user.Email)
		require.ErrorIs(t, err, domain.ErrNotFound)
		assert.Nil(t, got)
	})

	t.Run("Delete missing user", func(t *testing.T) {
		t.Parallel()

		store := newTestStore(t)
		repo := NewUserRepo(NewManager(store.db))
		ctx := t.Context()

		err := repo.Delete(ctx, "nonexistent@example.com")
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}
