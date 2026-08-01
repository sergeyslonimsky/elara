package bbolt_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/pkg/bbolt"
)

func TestBucket_TxBacked_GetPutDeleteForEach(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	mgr := bbolt.NewManager(db)

	err := mgr.WithTx(t.Context(), func(ctx context.Context) error {
		b := mgr.GetQuerier(ctx).Bucket("items")

		require.NoError(t, b.Put([]byte("a"), []byte("1")))
		require.NoError(t, b.Put([]byte("b"), []byte("2")))

		assert.Equal(t, []byte("1"), b.Get([]byte("a")))
		assert.Nil(t, b.Get([]byte("missing")))

		var seen [][2]string

		err := b.ForEach(func(k, v []byte) error {
			seen = append(seen, [2]string{string(k), string(v)})

			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, [][2]string{{"a", "1"}, {"b", "2"}}, seen)

		require.NoError(t, b.Delete([]byte("a")))
		assert.Nil(t, b.Get([]byte("a")))

		return nil
	})
	require.NoError(t, err)
}

func TestBucket_TxBacked_ForEach_AbortsOnError(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	mgr := bbolt.NewManager(db)

	err := mgr.WithTx(t.Context(), func(ctx context.Context) error {
		b := mgr.GetQuerier(ctx).Bucket("items")
		require.NoError(t, b.Put([]byte("a"), []byte("1")))

		return b.ForEach(func(_, _ []byte) error {
			return assert.AnError
		})
	})
	require.ErrorIs(t, err, assert.AnError)
	require.ErrorContains(t, err, "bbolt foreach:")
}

func TestBucket_TxBacked_Cursor(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	mgr := bbolt.NewManager(db)

	err := mgr.WithTx(t.Context(), func(ctx context.Context) error {
		b := mgr.GetQuerier(ctx).Bucket("items")
		for _, k := range []string{"a", "b", "c"} {
			require.NoError(t, b.Put([]byte(k), []byte(k)))
		}

		k, v := b.Cursor().First()
		assert.Equal(t, []byte("a"), k)
		assert.Equal(t, []byte("a"), v)

		k, v = b.Cursor().Last()
		assert.Equal(t, []byte("c"), k)
		assert.Equal(t, []byte("c"), v)

		c := b.Cursor()
		c.First()
		k, _ = c.Next()
		assert.Equal(t, []byte("b"), k)

		c = b.Cursor()
		c.Last()
		k, _ = c.Prev()
		assert.Equal(t, []byte("b"), k)

		k, v = b.Cursor().Seek([]byte("b"))
		assert.Equal(t, []byte("b"), k)
		assert.Equal(t, []byte("b"), v)

		return nil
	})
	require.NoError(t, err)
}

func TestBucket_AutoQuerier_GetPutDeleteForEach(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	mgr := bbolt.NewManager(db)

	b := mgr.GetQuerier(t.Context()).Bucket("items")

	require.NoError(t, b.Put([]byte("x"), []byte("1")))
	assert.Equal(t, []byte("1"), b.Get([]byte("x")))

	var seen [][2]string

	err := b.ForEach(func(k, v []byte) error {
		seen = append(seen, [2]string{string(k), string(v)})

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, [][2]string{{"x", "1"}}, seen)

	require.NoError(t, b.Delete([]byte("x")))
	assert.Nil(t, b.Get([]byte("x")))
}

func TestBucket_AutoQuerier_MissingBucket(t *testing.T) {
	t.Parallel()

	db := newTestDB(t) // no buckets created
	mgr := bbolt.NewManager(db)

	b := mgr.GetQuerier(t.Context()).Bucket("nope")

	assert.Nil(t, b.Get([]byte("x")))

	err := b.Put([]byte("x"), []byte("1"))
	require.ErrorIs(t, err, bbolt.ErrBucketNotFound)

	err = b.Delete([]byte("x"))
	require.ErrorIs(t, err, bbolt.ErrBucketNotFound)

	err = b.ForEach(func(_, _ []byte) error { return nil })
	require.ErrorIs(t, err, bbolt.ErrBucketNotFound)
}

func TestBucket_AutoQuerier_CursorUnsupported(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	mgr := bbolt.NewManager(db)

	b := mgr.GetQuerier(t.Context()).Bucket("items")
	c := b.Cursor()

	k, v := c.First()
	assert.Nil(t, k)
	assert.Nil(t, v)

	k, v = c.Last()
	assert.Nil(t, k)
	assert.Nil(t, v)

	k, v = c.Next()
	assert.Nil(t, k)
	assert.Nil(t, v)

	k, v = c.Prev()
	assert.Nil(t, k)
	assert.Nil(t, v)

	k, v = c.Seek([]byte("x"))
	assert.Nil(t, k)
	assert.Nil(t, v)
}
