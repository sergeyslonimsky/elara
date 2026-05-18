package bbolt

import (
	"context"
	"fmt"

	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

type TxManager struct {
	db *bolt.DB
}

// NewTxManager creates a new TxManager backed by bbolt.
func NewTxManager(db *bolt.DB) *TxManager {
	return &TxManager{db: db}
}

func (m *TxManager) Read(_ context.Context, fn func(storage.Tx) error) error {
	if err := m.db.View(func(tx *bolt.Tx) error {
		return fn(&txWrapper{tx: tx})
	}); err != nil {
		return fmt.Errorf("bbolt read: %w", err)
	}

	return nil
}

func (m *TxManager) Write(_ context.Context, fn func(storage.Tx) error) error {
	if err := m.db.Update(func(tx *bolt.Tx) error {
		return fn(&txWrapper{tx: tx})
	}); err != nil {
		return fmt.Errorf("bbolt write: %w", err)
	}

	return nil
}

type txWrapper struct {
	tx *bolt.Tx
}

func (w *txWrapper) Bucket(name []byte) storage.Bucket {
	b := w.tx.Bucket(name)
	if b == nil {
		return nil
	}

	return &bucketWrapper{b: b}
}

type bucketWrapper struct {
	b *bolt.Bucket
}

func (w *bucketWrapper) Get(key []byte) []byte {
	return w.b.Get(key)
}

func (w *bucketWrapper) Put(key, value []byte) error {
	return w.b.Put(key, value)
}

func (w *bucketWrapper) Delete(key []byte) error {
	return w.b.Delete(key)
}

func (w *bucketWrapper) ForEach(fn func(k, v []byte) error) error {
	return w.b.ForEach(fn)
}
