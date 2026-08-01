package bbolt_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/pkg/bbolt"
)

func TestQuerier_TxBacked_MissingBucket(t *testing.T) {
	t.Parallel()

	db := newTestDB(t) // no buckets created
	mgr := bbolt.NewManager(db)

	err := mgr.WithTx(t.Context(), func(ctx context.Context) error {
		b := mgr.GetQuerier(ctx).Bucket("nope")

		assert.Nil(t, b.Get([]byte("k")))

		putErr := b.Put([]byte("k"), []byte("v"))
		require.ErrorIs(t, putErr, bbolt.ErrBucketNotFound)

		delErr := b.Delete([]byte("k"))
		require.ErrorIs(t, delErr, bbolt.ErrBucketNotFound)

		feErr := b.ForEach(func(_, _ []byte) error { return nil })
		require.ErrorIs(t, feErr, bbolt.ErrBucketNotFound)

		k, v := b.Cursor().First()
		assert.Nil(t, k)
		assert.Nil(t, v)

		return nil
	})
	require.NoError(t, err)
}

func TestQuerier_TxBacked_ExistingBucket(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	mgr := bbolt.NewManager(db)

	err := mgr.WithTx(t.Context(), func(ctx context.Context) error {
		b := mgr.GetQuerier(ctx).Bucket("items")
		require.NoError(t, b.Put([]byte("a"), []byte("1")))

		return nil
	})
	require.NoError(t, err)
}

func TestQuerier_AutoQuerier_Bucket(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	mgr := bbolt.NewManager(db)

	b := mgr.GetQuerier(t.Context()).Bucket("items")
	require.NoError(t, b.Put([]byte("a"), []byte("1")))
	assert.Equal(t, []byte("1"), b.Get([]byte("a")))
}
