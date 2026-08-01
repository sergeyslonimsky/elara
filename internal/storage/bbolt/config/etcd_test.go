package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestRepository_PutKey_AndRangeQuery(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	prev, rev, err := repo.PutKey(ctx, "ns", "/a", []byte("v1"))
	require.NoError(t, err)
	assert.Nil(t, prev)
	assert.Equal(t, int64(1), rev)

	results, more, err := repo.RangeQuery(ctx, "ns", "/a", "", "", 0, 0, false)
	require.NoError(t, err)
	assert.False(t, more)
	require.Len(t, results, 1)
	assert.Equal(t, []byte("v1"), results[0].Value)
	assert.Equal(t, "/a", results[0].Path)
	assert.Equal(t, int64(1), results[0].Version)

	_, _, err = repo.PutKey(ctx, "ns", "/b", []byte("v1"))
	require.NoError(t, err)
	_, _, err = repo.PutKey(ctx, "ns", "/c", []byte("v1"))
	require.NoError(t, err)

	results, _, err = repo.RangeQuery(ctx, "ns", "/a", "ns", "/d", 0, 0, false)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestRepository_PutKey_UpdatesExisting(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	_, _, err := repo.PutKey(ctx, "ns", "/a", []byte("v1"))
	require.NoError(t, err)

	prev, rev, err := repo.PutKey(ctx, "ns", "/a", []byte("v2"))
	require.NoError(t, err)
	require.NotNil(t, prev)
	assert.Equal(t, []byte("v1"), prev.Value)
	assert.Equal(t, int64(2), rev)

	results, _, err := repo.RangeQuery(ctx, "ns", "/a", "", "", 0, 0, false)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, []byte("v2"), results[0].Value)
	assert.Equal(t, int64(2), results[0].Version)
}

func TestRepository_DeleteRangeKeys(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	for _, p := range []string{"/a", "/b", "/c"} {
		_, _, err := repo.PutKey(ctx, "ns", p, []byte("v"))
		require.NoError(t, err)
	}

	deleted, rev, err := repo.DeleteRangeKeys(ctx, "ns", "/a", "ns", "/c", true)
	require.NoError(t, err)
	assert.Positive(t, rev)
	assert.Len(t, deleted, 2)

	results, _, err := repo.RangeQuery(ctx, "ns", "/a", "ns", "/c", 0, 0, false)
	require.NoError(t, err)
	assert.Empty(t, results)

	results, _, err = repo.RangeQuery(ctx, "ns", "/c", "", "", 0, 0, false)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestRepository_PutKey_LockedConfig(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	_, _, err := repo.PutKey(ctx, "ns", "/a", []byte("v1"))
	require.NoError(t, err)
	require.NoError(t, repo.LockConfig(ctx, "ns", "/a"))

	_, _, err = repo.PutKey(ctx, "ns", "/a", []byte("v2"))
	require.ErrorIs(t, err, domain.ErrLocked)
}

func TestRepository_DeleteRangeKeys_NoMatches(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	deleted, rev, err := repo.DeleteRangeKeys(ctx, "ns", "/missing", "ns", "/missing-end", true)
	require.NoError(t, err)
	assert.Zero(t, rev)
	assert.Empty(t, deleted)
}

func TestRepository_DeleteRangeKeys_LockedConfig(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	_, _, err := repo.PutKey(ctx, "ns", "/a", []byte("v"))
	require.NoError(t, err)
	require.NoError(t, repo.LockConfig(ctx, "ns", "/a"))

	_, _, err = repo.DeleteRangeKeys(ctx, "ns", "/a", "", "", true)
	require.ErrorIs(t, err, domain.ErrLocked)
}

func TestRepository_RangeQuery_KeysOnly(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	_, _, err := repo.PutKey(ctx, "ns", "/a", []byte("v1"))
	require.NoError(t, err)

	results, _, err := repo.RangeQuery(ctx, "ns", "/a", "", "", 0, 0, true)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Nil(t, results[0].Value)
	assert.Equal(t, "/a", results[0].Path)
}

func TestRepository_RangeQuery_AtRevision(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	_, _, err := repo.PutKey(ctx, "ns", "/a", []byte("v1"))
	require.NoError(t, err)
	_, _, err = repo.PutKey(ctx, "ns", "/a", []byte("v2"))
	require.NoError(t, err)

	results, _, err := repo.RangeQuery(ctx, "ns", "/a", "", "", 0, 1, false)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, []byte("v1"), results[0].Value)
	assert.Equal(t, int64(1), results[0].ModRevision)
}

func TestRepository_RangeQuery_Limit_SetsMore(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	for _, p := range []string{"/a", "/b", "/c"} {
		_, _, err := repo.PutKey(ctx, "ns", p, []byte("v"))
		require.NoError(t, err)
	}

	results, more, err := repo.RangeQuery(ctx, "ns", "/a", "ns", "/z", 2, 0, false)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.True(t, more)
}

func TestRepository_CurrentRevisionValue(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	rev, err := repo.CurrentRevisionValue(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rev)

	_, _, err = repo.PutKey(ctx, "ns", "/a", []byte("v"))
	require.NoError(t, err)

	rev, err = repo.CurrentRevisionValue(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rev)
}

func TestRepository_GetKVAtRevision(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	_, _, err := repo.PutKey(ctx, "ns", "/a", []byte("v1"))
	require.NoError(t, err)
	_, _, err = repo.PutKey(ctx, "ns", "/a", []byte("v2"))
	require.NoError(t, err)

	got, err := repo.GetKVAtRevision(ctx, "ns", "/a", 1)
	require.NoError(t, err)
	assert.Equal(t, []byte("v1"), got)

	got, err = repo.GetKVAtRevision(ctx, "ns", "/a", 2)
	require.NoError(t, err)
	assert.Equal(t, []byte("v2"), got)

	got, err = repo.GetKVAtRevision(ctx, "ns", "/missing", 1)
	require.NoError(t, err)
	assert.Nil(t, got)
}
