package bbolt_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/pkg/bbolt"
)

func TestDBManager_WithTx_CommitsOnSuccess(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	mgr := bbolt.NewManager(db)

	err := mgr.WithTx(t.Context(), func(ctx context.Context) error {
		return bbolt.Put(mgr.GetQuerier(ctx), "items", []byte("a"), testItem{ID: "a", Value: 1})
	})
	require.NoError(t, err)

	got, err := bbolt.Get[testItem](mgr.GetQuerier(t.Context()), "items", []byte("a"))
	require.NoError(t, err)
	assert.Equal(t, testItem{ID: "a", Value: 1}, got)
}

func TestDBManager_WithTx_RollsBackOnError(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	mgr := bbolt.NewManager(db)

	sentinel := assert.AnError

	err := mgr.WithTx(t.Context(), func(ctx context.Context) error {
		if putErr := bbolt.Put(mgr.GetQuerier(ctx), "items", []byte("a"), testItem{ID: "a"}); putErr != nil {
			return putErr
		}

		return sentinel
	})
	require.ErrorIs(t, err, sentinel)

	_, getErr := bbolt.Get[testItem](mgr.GetQuerier(t.Context()), "items", []byte("a"))
	require.ErrorIs(t, getErr, bbolt.ErrNotFound)
}

func TestDBManager_WithTx_PanicRollsBack(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	mgr := bbolt.NewManager(db)

	assert.Panics(t, func() {
		_ = mgr.WithTx(t.Context(), func(ctx context.Context) error {
			_ = bbolt.Put(mgr.GetQuerier(ctx), "items", []byte("a"), testItem{ID: "a"})
			panic("boom")
		})
	})

	_, getErr := bbolt.Get[testItem](mgr.GetQuerier(t.Context()), "items", []byte("a"))
	require.ErrorIs(t, getErr, bbolt.ErrNotFound)
}

func TestDBManager_WithTx_NestedFlattensSameTx(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	mgr := bbolt.NewManager(db)

	var outerTx, innerTx *bolt.Tx

	err := mgr.WithTx(t.Context(), func(outerCtx context.Context) error {
		tx, ok := bbolt.TxFromContext(outerCtx)
		require.True(t, ok)
		outerTx = tx

		return mgr.WithTx(outerCtx, func(innerCtx context.Context) error {
			tx, ok := bbolt.TxFromContext(innerCtx)
			require.True(t, ok)
			innerTx = tx

			return nil
		})
	})
	require.NoError(t, err)
	assert.Same(t, outerTx, innerTx)
}

func TestDBManager_WithTx_BeginError(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	mgr := bbolt.NewManager(db)
	require.NoError(t, db.Close())

	err := mgr.WithTx(t.Context(), func(context.Context) error {
		return nil
	})
	require.ErrorContains(t, err, "begin tx:")
}

func TestDBManager_WithReadTx_RejectsWrites(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	mgr := bbolt.NewManager(db)

	err := mgr.WithReadTx(t.Context(), func(ctx context.Context) error {
		return bbolt.Put(mgr.GetQuerier(ctx), "items", []byte("a"), testItem{ID: "a"})
	})
	require.Error(t, err)
}

func TestDBManager_WithReadTx_NestedInsideWithTx(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	mgr := bbolt.NewManager(db)

	err := mgr.WithTx(t.Context(), func(outerCtx context.Context) error {
		return mgr.WithReadTx(outerCtx, func(innerCtx context.Context) error {
			return bbolt.Put(mgr.GetQuerier(innerCtx), "items", []byte("a"), testItem{ID: "a"})
		})
	})
	require.NoError(t, err)

	got, err := bbolt.Get[testItem](mgr.GetQuerier(t.Context()), "items", []byte("a"))
	require.NoError(t, err)
	assert.Equal(t, testItem{ID: "a"}, got)
}

func TestDBManager_WithReadTx_BeginError(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	mgr := bbolt.NewManager(db)
	require.NoError(t, db.Close())

	err := mgr.WithReadTx(t.Context(), func(context.Context) error {
		return nil
	})
	require.ErrorContains(t, err, "begin read tx:")
}

func TestTxFromContext_NoTxInContext(t *testing.T) {
	t.Parallel()

	tx, ok := bbolt.TxFromContext(t.Context())
	assert.False(t, ok)
	assert.Nil(t, tx)
}

func TestDBManager_DB(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	mgr := bbolt.NewManager(db)

	assert.Same(t, db, mgr.DB())
}

func TestDBManager_GetQuerier_AutoWhenNoTx(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	mgr := bbolt.NewManager(db)

	err := bbolt.Put(mgr.GetQuerier(t.Context()), "items", []byte("a"), testItem{ID: "a", Value: 1})
	require.NoError(t, err)

	got, err := bbolt.Get[testItem](mgr.GetQuerier(t.Context()), "items", []byte("a"))
	require.NoError(t, err)
	assert.Equal(t, testItem{ID: "a", Value: 1}, got)
}
