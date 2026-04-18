package bbolt_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bboltadapter "github.com/sergeyslonimsky/elara/internal/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

// -----------------------------------------------------------------------------
// PutKey
// -----------------------------------------------------------------------------

func TestConfigRepo_PutKey_CreatesNewKey(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	prev, newRev, err := repo.PutKey(ctx, "default", "/foo.json", []byte(`{"x":1}`))
	require.NoError(t, err)
	assert.Nil(t, prev, "no prev on first put")
	assert.Equal(t, int64(1), newRev)

	got, err := repo.Get(ctx, "/foo.json", "default")
	require.NoError(t, err)
	assert.Equal(t, `{"x":1}`, got.Content)
	assert.Equal(t, int64(1), got.Version)
	assert.Equal(t, int64(1), got.Revision)
	assert.Equal(t, int64(1), got.CreateRevision)
}

func TestConfigRepo_PutKey_UpdatesExistingKey(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	_, _, err := repo.PutKey(ctx, "default", "/foo", []byte("v1"))
	require.NoError(t, err)

	prev, newRev, err := repo.PutKey(ctx, "default", "/foo", []byte("v2"))
	require.NoError(t, err)
	require.NotNil(t, prev)
	assert.Equal(t, []byte("v1"), prev.Value)
	assert.Equal(t, int64(1), prev.Version)
	assert.Equal(t, int64(1), prev.ModRevision)
	assert.Equal(t, int64(2), newRev)

	got, err := repo.Get(ctx, "/foo", "default")
	require.NoError(t, err)
	assert.Equal(t, "v2", got.Content)
	assert.Equal(t, int64(2), got.Version)
	assert.Equal(t, int64(2), got.Revision)
	assert.Equal(t, int64(1), got.CreateRevision, "CreateRevision preserved across updates")
}

func TestConfigRepo_PutKey_IndependentKeys(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	_, rev1, err := repo.PutKey(ctx, "default", "/a", []byte("a"))
	require.NoError(t, err)

	_, rev2, err := repo.PutKey(ctx, "default", "/b", []byte("b"))
	require.NoError(t, err)

	assert.Equal(t, int64(1), rev1)
	assert.Equal(t, int64(2), rev2, "global revision monotonic across keys")
}

// -----------------------------------------------------------------------------
// RangeQuery
// -----------------------------------------------------------------------------

func TestConfigRepo_RangeQuery_SingleKey(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	_, _, err := repo.PutKey(ctx, "default", "/foo", []byte("v1"))
	require.NoError(t, err)

	kvs, more, err := repo.RangeQuery(ctx, "default", "/foo", "", "", 0, 0, false)
	require.NoError(t, err)
	require.Len(t, kvs, 1)
	assert.Equal(t, "default", kvs[0].Namespace)
	assert.Equal(t, "/foo", kvs[0].Path)
	assert.Equal(t, []byte("v1"), kvs[0].Value)
	assert.False(t, more)
}

func TestConfigRepo_RangeQuery_SingleKey_Missing(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)

	kvs, more, err := repo.RangeQuery(context.Background(), "default", "/missing", "", "", 0, 0, false)
	require.NoError(t, err)
	assert.Empty(t, kvs)
	assert.False(t, more)
}

func TestConfigRepo_RangeQuery_ExplicitRange(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	for _, p := range []string{"/a", "/b", "/c", "/d"} {
		_, _, err := repo.PutKey(ctx, "default", p, []byte("v"))
		require.NoError(t, err)
	}

	kvs, _, err := repo.RangeQuery(ctx, "default", "/b", "default", "/d", 0, 0, false)
	require.NoError(t, err)
	require.Len(t, kvs, 2, "[/b, /d) → /b, /c")
	assert.Equal(t, "/b", kvs[0].Path)
	assert.Equal(t, "/c", kvs[1].Path)
}

func TestConfigRepo_RangeQuery_ScanAll(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	_, _, err := repo.PutKey(ctx, "default", "/a", []byte("a"))
	require.NoError(t, err)
	_, _, err = repo.PutKey(ctx, "prod", "/x", []byte("x"))
	require.NoError(t, err)

	kvs, _, err := repo.RangeQuery(ctx, "default", "/", "\x00", "", 0, 0, false)
	require.NoError(t, err)
	assert.Len(t, kvs, 2, "scan-all returns every key >= start across all namespaces")
}

func TestConfigRepo_RangeQuery_Limit_ReportsMore(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	for _, p := range []string{"/a", "/b", "/c"} {
		_, _, err := repo.PutKey(ctx, "default", p, []byte("v"))
		require.NoError(t, err)
	}

	kvs, more, err := repo.RangeQuery(ctx, "default", "/", "default", "/z", 2, 0, false)
	require.NoError(t, err)
	assert.Len(t, kvs, 2)
	assert.True(t, more)
}

func TestConfigRepo_RangeQuery_Limit_NoMoreOnExactFit(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	for _, p := range []string{"/a", "/b"} {
		_, _, err := repo.PutKey(ctx, "default", p, []byte("v"))
		require.NoError(t, err)
	}

	kvs, more, err := repo.RangeQuery(ctx, "default", "/", "default", "/z", 2, 0, false)
	require.NoError(t, err)
	assert.Len(t, kvs, 2)
	assert.False(t, more, "exactly limit entries and nothing else → more=false")
}

func TestConfigRepo_RangeQuery_KeysOnly(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	_, _, err := repo.PutKey(ctx, "default", "/foo", []byte("hello"))
	require.NoError(t, err)

	kvs, _, err := repo.RangeQuery(ctx, "default", "/foo", "", "", 0, 0, true)
	require.NoError(t, err)
	require.Len(t, kvs, 1)
	assert.Empty(t, kvs[0].Value, "keysOnly must strip value")
	assert.Equal(t, int64(1), kvs[0].Version, "metadata still populated")
}

func TestConfigRepo_RangeQuery_PointInTime(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	_, _, err := repo.PutKey(ctx, "default", "/foo", []byte("v1"))
	require.NoError(t, err)
	_, _, err = repo.PutKey(ctx, "default", "/foo", []byte("v2"))
	require.NoError(t, err)
	_, _, err = repo.PutKey(ctx, "default", "/foo", []byte("v3"))
	require.NoError(t, err)

	// Read at rev=2 — should see "v2"
	kvs, _, err := repo.RangeQuery(ctx, "default", "/foo", "", "", 0, 2, false)
	require.NoError(t, err)
	require.Len(t, kvs, 1)
	assert.Equal(t, []byte("v2"), kvs[0].Value)
	assert.Equal(t, int64(2), kvs[0].ModRevision)

	// Read at rev=1 — should see "v1"
	kvs, _, err = repo.RangeQuery(ctx, "default", "/foo", "", "", 0, 1, false)
	require.NoError(t, err)
	require.Len(t, kvs, 1)
	assert.Equal(t, []byte("v1"), kvs[0].Value)
}

func TestConfigRepo_RangeQuery_PointInTime_BeforeKeyExisted(t *testing.T) {
	t.Parallel()
	// Regression guard for C2: when historical lookup returns nil (key didn't
	// exist at rev), the cursor must continue to the next key, not skip one.
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	_, _, err := repo.PutKey(ctx, "default", "/a", []byte("a-v1"))
	require.NoError(t, err)
	// rev=1: only /a exists

	_, _, err = repo.PutKey(ctx, "default", "/b", []byte("b-v1"))
	require.NoError(t, err)
	_, _, err = repo.PutKey(ctx, "default", "/c", []byte("c-v1"))
	require.NoError(t, err)

	// Read range at rev=1: /a should exist, /b and /c didn't yet.
	kvs, _, err := repo.RangeQuery(ctx, "default", "/", "default", "/z", 0, 1, false)
	require.NoError(t, err)
	require.Len(t, kvs, 1, "/b and /c did not exist at rev=1")
	assert.Equal(t, "/a", kvs[0].Path)
}

func TestConfigRepo_RangeQuery_ValueIsCopy(t *testing.T) {
	t.Parallel()
	// Regression guard for C1: returned Value must not alias bbolt's mmap.
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	_, _, err := repo.PutKey(ctx, "default", "/foo", []byte("original"))
	require.NoError(t, err)

	kvs, _, err := repo.RangeQuery(ctx, "default", "/foo", "", "", 0, 0, false)
	require.NoError(t, err)
	require.Len(t, kvs, 1)

	// Mutate the returned slice — must not affect storage.
	kvs[0].Value[0] = 'X'

	kvs2, _, err := repo.RangeQuery(ctx, "default", "/foo", "", "", 0, 0, false)
	require.NoError(t, err)
	require.Len(t, kvs2, 1)
	assert.Equal(t, []byte("original"), kvs2[0].Value, "storage must be unaffected by caller mutation")
}

// -----------------------------------------------------------------------------
// DeleteRangeKeys
// -----------------------------------------------------------------------------

func TestConfigRepo_DeleteRangeKeys_Single(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	_, _, err := repo.PutKey(ctx, "default", "/foo", []byte("v"))
	require.NoError(t, err)

	deleted, newRev, err := repo.DeleteRangeKeys(ctx, "default", "/foo", "", "", true)
	require.NoError(t, err)
	require.Len(t, deleted, 1)
	assert.Equal(t, []byte("v"), deleted[0].Value)
	assert.Equal(t, int64(2), newRev)

	_, err = repo.Get(ctx, "/foo", "default")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestConfigRepo_DeleteRangeKeys_NoPrev(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	_, _, err := repo.PutKey(ctx, "default", "/foo", []byte("v"))
	require.NoError(t, err)

	deleted, _, err := repo.DeleteRangeKeys(ctx, "default", "/foo", "", "", false)
	require.NoError(t, err)
	require.Len(t, deleted, 1)
	assert.Nil(t, deleted[0].Value, "returnPrev=false must not return values")
	assert.Equal(t, "default", deleted[0].Namespace)
	assert.Equal(t, "/foo", deleted[0].Path)
}

func TestConfigRepo_DeleteRangeKeys_NothingMatches(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)

	deleted, newRev, err := repo.DeleteRangeKeys(context.Background(), "default", "/missing", "", "", false)
	require.NoError(t, err)
	assert.Empty(t, deleted)
	assert.Equal(t, int64(0), newRev, "nothing deleted → revision unchanged (0)")
}

func TestConfigRepo_DeleteRangeKeys_Range(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	for _, p := range []string{"/a", "/b", "/c", "/d"} {
		_, _, err := repo.PutKey(ctx, "default", p, []byte("v"))
		require.NoError(t, err)
	}

	deleted, newRev, err := repo.DeleteRangeKeys(ctx, "default", "/b", "default", "/d", true)
	require.NoError(t, err)
	assert.Len(t, deleted, 2, "[/b, /d) → 2 keys")
	assert.Equal(t, int64(5), newRev, "single revision for whole batch")

	// /a and /d still present
	kvs, _, err := repo.RangeQuery(ctx, "default", "/", "default", "/z", 0, 0, false)
	require.NoError(t, err)
	require.Len(t, kvs, 2)
	assert.Equal(t, "/a", kvs[0].Path)
	assert.Equal(t, "/d", kvs[1].Path)
}

func TestConfigRepo_DeleteRangeKeys_ScanAll(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	for _, p := range []string{"/a", "/b"} {
		_, _, err := repo.PutKey(ctx, "default", p, []byte("v"))
		require.NoError(t, err)
	}

	_, _, err := repo.PutKey(ctx, "prod", "/x", []byte("v"))
	require.NoError(t, err)

	deleted, _, err := repo.DeleteRangeKeys(ctx, "default", "/", "\x00", "", false)
	require.NoError(t, err)
	assert.Len(t, deleted, 3, "scan-all deletes everything >= start across namespaces")
}

// -----------------------------------------------------------------------------
// GetKVAtRevision + CurrentRevisionValue
// -----------------------------------------------------------------------------

func TestConfigRepo_GetKVAtRevision(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	_, _, err := repo.PutKey(ctx, "default", "/foo", []byte("v1"))
	require.NoError(t, err)
	_, _, err = repo.PutKey(ctx, "default", "/foo", []byte("v2"))
	require.NoError(t, err)

	v1, err := repo.GetKVAtRevision(ctx, "default", "/foo", 1)
	require.NoError(t, err)
	assert.Equal(t, []byte("v1"), v1)

	v2, err := repo.GetKVAtRevision(ctx, "default", "/foo", 2)
	require.NoError(t, err)
	assert.Equal(t, []byte("v2"), v2)

	// Reading at a revision later than last write returns the most recent history entry.
	v3, err := repo.GetKVAtRevision(ctx, "default", "/foo", 99)
	require.NoError(t, err)
	assert.Equal(t, []byte("v2"), v3)
}

func TestConfigRepo_GetKVAtRevision_Missing(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)

	got, err := repo.GetKVAtRevision(context.Background(), "ns", "/missing", 1)
	require.NoError(t, err)
	assert.Nil(t, got, "unknown key returns nil, no error")
}

func TestConfigRepo_GetKVAtRevision_BeforeKeyExisted(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	// Bump revision with an unrelated put first.
	_, _, err := repo.PutKey(ctx, "default", "/other", []byte("x"))
	require.NoError(t, err)
	_, _, err = repo.PutKey(ctx, "default", "/other", []byte("y"))
	require.NoError(t, err)

	// Now create /foo at rev=3
	_, _, err = repo.PutKey(ctx, "default", "/foo", []byte("v1"))
	require.NoError(t, err)

	// Requesting /foo at rev=1 should return nil (didn't exist yet).
	got, err := repo.GetKVAtRevision(ctx, "default", "/foo", 1)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestConfigRepo_GetKVAtRevision_IsCopy(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	_, _, err := repo.PutKey(ctx, "default", "/foo", []byte("original"))
	require.NoError(t, err)

	got, err := repo.GetKVAtRevision(ctx, "default", "/foo", 1)
	require.NoError(t, err)
	require.NotNil(t, got)

	got[0] = 'X'

	got2, err := repo.GetKVAtRevision(ctx, "default", "/foo", 1)
	require.NoError(t, err)
	assert.Equal(t, []byte("original"), got2, "returned bytes must be a copy")
}

func TestConfigRepo_CurrentRevisionValue(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	rev, err := repo.CurrentRevisionValue(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rev, "fresh store")

	_, _, err = repo.PutKey(ctx, "default", "/a", []byte("v"))
	require.NoError(t, err)

	rev, err = repo.CurrentRevisionValue(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rev)

	_, _, err = repo.PutKey(ctx, "default", "/b", []byte("v"))
	require.NoError(t, err)

	rev, err = repo.CurrentRevisionValue(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), rev)
}

// -----------------------------------------------------------------------------
// Cross-functional: PutKey + RangeQuery + DeleteRangeKeys integration
// -----------------------------------------------------------------------------

func TestConfigRepo_Etcd_RoundTrip(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewConfigRepo(store)
	ctx := context.Background()

	// Create
	_, rev1, err := repo.PutKey(ctx, "default", "/foo", []byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, int64(1), rev1)

	// Update (check CreateRevision preservation)
	prev, rev2, err := repo.PutKey(ctx, "default", "/foo", []byte("world"))
	require.NoError(t, err)
	require.NotNil(t, prev)
	assert.Equal(t, int64(2), rev2)
	assert.Equal(t, int64(1), prev.CreateRevision)

	kvs, _, err := repo.RangeQuery(ctx, "default", "/foo", "", "", 0, 0, false)
	require.NoError(t, err)
	require.Len(t, kvs, 1)
	assert.Equal(t, int64(1), kvs[0].CreateRevision)
	assert.Equal(t, int64(2), kvs[0].ModRevision)
	assert.Equal(t, int64(2), kvs[0].Version)
	assert.Equal(t, []byte("world"), kvs[0].Value)

	// Point-in-time at rev=1 returns the original value.
	kvs, _, err = repo.RangeQuery(ctx, "default", "/foo", "", "", 0, 1, false)
	require.NoError(t, err)
	require.Len(t, kvs, 1)
	assert.Equal(t, []byte("hello"), kvs[0].Value)

	// Delete
	deleted, rev3, err := repo.DeleteRangeKeys(ctx, "default", "/foo", "", "", true)
	require.NoError(t, err)
	require.Len(t, deleted, 1)
	assert.Equal(t, []byte("world"), deleted[0].Value)
	assert.Equal(t, int64(3), rev3)

	// Gone from live view
	kvs, _, err = repo.RangeQuery(ctx, "default", "/foo", "", "", 0, 0, false)
	require.NoError(t, err)
	assert.Empty(t, kvs)

	// Historical reads still work post-delete (history bucket is not compacted).
	v, err := repo.GetKVAtRevision(ctx, "default", "/foo", 2)
	require.NoError(t, err)
	assert.Equal(t, []byte("world"), v)
}
