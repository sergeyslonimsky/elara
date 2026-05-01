package bbolt_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bboltadapter "github.com/sergeyslonimsky/elara/internal/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestUserRepo_Upsert_New(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(store)
	ctx := t.Context()

	user := &domain.User{
		Email:       "alice@example.com",
		Name:        "Alice",
		Picture:     "https://example.com/alice.png",
		Provider:    "oidc",
		LastLoginAt: time.Now(),
	}

	err := repo.Upsert(ctx, user)
	require.NoError(t, err)
	assert.False(t, user.CreatedAt.IsZero(), "CreatedAt should be set after first upsert")

	got, err := repo.Get(ctx, user.Email)
	require.NoError(t, err)
	assert.Equal(t, user.Email, got.Email)
	assert.Equal(t, user.Name, got.Name)
	assert.Equal(t, user.Provider, got.Provider)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestUserRepo_Upsert_Existing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(store)
	ctx := t.Context()

	user := &domain.User{
		Email:       "bob@example.com",
		Name:        "Bob",
		Provider:    "oidc",
		LastLoginAt: time.Now(),
	}
	require.NoError(t, repo.Upsert(ctx, user))

	originalCreatedAt := user.CreatedAt

	// Update name and last login.
	user.Name = "Bob Updated"
	user.LastLoginAt = time.Now().Add(time.Hour)
	require.NoError(t, repo.Upsert(ctx, user))

	got, err := repo.Get(ctx, user.Email)
	require.NoError(t, err)
	assert.Equal(t, "Bob Updated", got.Name)
	assert.Equal(t, originalCreatedAt.UnixNano(), got.CreatedAt.UnixNano(), "CreatedAt must not change on update")
}

func TestUserRepo_Get_Existing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(store)
	ctx := t.Context()

	user := &domain.User{Email: "carol@example.com", Name: "Carol", Provider: "oidc", LastLoginAt: time.Now()}
	require.NoError(t, repo.Upsert(ctx, user))

	got, err := repo.Get(ctx, "carol@example.com")
	require.NoError(t, err)
	assert.Equal(t, "carol@example.com", got.Email)
	assert.Equal(t, "Carol", got.Name)
}

func TestUserRepo_Get_Missing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(store)
	ctx := t.Context()

	_, err := repo.Get(ctx, "nobody@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestUserRepo_List_Empty(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(store)
	ctx := t.Context()

	users, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, users)
}

func TestUserRepo_List_Multiple(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(store)
	ctx := t.Context()

	emails := []string{"dave@example.com", "eve@example.com", "frank@example.com"}
	for _, email := range emails {
		u := &domain.User{Email: email, Name: email, Provider: "oidc", LastLoginAt: time.Now()}
		require.NoError(t, repo.Upsert(ctx, u))
	}

	users, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, users, len(emails))
}
