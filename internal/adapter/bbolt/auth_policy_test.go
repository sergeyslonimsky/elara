package bbolt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bboltadapter "github.com/sergeyslonimsky/elara/internal/adapter/bbolt"
)

func TestPolicyRepo_Load_Empty(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPolicyRepo(store)
	ctx := t.Context()

	rules, err := repo.Load(ctx)
	require.NoError(t, err)
	assert.NotNil(t, rules, "Load on empty store must return empty slice, not nil")
	assert.Empty(t, rules)
}

func TestPolicyRepo_Save_And_Load(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPolicyRepo(store)
	ctx := t.Context()

	rules := [][]string{
		{"p", "alice", "data1", "read"},
		{"p", "bob", "data2", "write"},
		{"g", "alice", "admin"},
	}

	require.NoError(t, repo.Save(ctx, rules))

	loaded, err := repo.Load(ctx)
	require.NoError(t, err)
	assert.Equal(t, rules, loaded)
}

func TestPolicyRepo_Save_Overwrites(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPolicyRepo(store)
	ctx := t.Context()

	initial := [][]string{
		{"p", "alice", "data1", "read"},
	}
	require.NoError(t, repo.Save(ctx, initial))

	updated := [][]string{
		{"p", "carol", "data3", "write"},
		{"p", "dave", "data4", "read"},
	}
	require.NoError(t, repo.Save(ctx, updated))

	loaded, err := repo.Load(ctx)
	require.NoError(t, err)
	assert.Equal(t, updated, loaded, "Save should overwrite the previous policy")
}
