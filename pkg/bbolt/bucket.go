package bbolt

import (
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// Bucket is the minimal key-value interface exposed to repositories. It is
// backend-agnostic in shape — though implemented by bbolt, the same surface
// could be backed by any embedded KV store.
//
// All methods accept raw byte slices. Higher-level helpers (Get[T], Put[T],
// List[T] in executor.go) layer JSON or pluggable codec encoding on top.
type Bucket interface {
	// Get returns the value for key, or nil if the key is not present.
	// The returned slice is valid only for the lifetime of the underlying
	// transaction. Callers MUST copy it if they need to retain the bytes
	// beyond the current tx (or beyond the current op for autoQuerier).
	Get(key []byte) []byte

	// Put stores value at key. Overwrites any existing value.
	Put(key, value []byte) error

	// Delete removes the key. Returns nil if the key did not exist.
	Delete(key []byte) error

	// ForEach iterates over all key/value pairs in the bucket in key order.
	// Returning a non-nil error from fn aborts iteration and returns that
	// error from ForEach.
	ForEach(fn func(k, v []byte) error) error

	// Cursor returns a cursor positioned before the first key.
	Cursor() Cursor
}

// Cursor traverses keys in a bucket. Cursors are valid only for the lifetime
// of the underlying transaction.
type Cursor interface {
	// First positions the cursor at the first key and returns it.
	First() (key, value []byte)

	// Last positions the cursor at the last key and returns it.
	Last() (key, value []byte)

	// Next advances the cursor to the next key.
	Next() (key, value []byte)

	// Prev moves the cursor to the previous key.
	Prev() (key, value []byte)

	// Seek positions the cursor at the first key >= prefix.
	Seek(prefix []byte) (key, value []byte)
}

// txBucket is a Bucket backed by a single *bolt.Bucket obtained from an
// explicit transaction. Operations execute directly on the underlying tx.
type txBucket struct {
	b *bolt.Bucket
}

func (t txBucket) Get(key []byte) []byte { return t.b.Get(key) }

func (t txBucket) Put(key, value []byte) error {
	if err := t.b.Put(key, value); err != nil {
		return fmt.Errorf("bbolt put: %w", err)
	}

	return nil
}

func (t txBucket) Delete(key []byte) error {
	if err := t.b.Delete(key); err != nil {
		return fmt.Errorf("bbolt delete: %w", err)
	}

	return nil
}

func (t txBucket) ForEach(fn func(k, v []byte) error) error {
	if err := t.b.ForEach(fn); err != nil {
		return fmt.Errorf("bbolt foreach: %w", err)
	}

	return nil
}

func (t txBucket) Cursor() Cursor { return boltCursor{c: t.b.Cursor()} }

// autoBucket is a Bucket that opens a fresh bbolt tx for every operation.
// It is the analogue of using *sql.DB (rather than *sql.Tx) as a Querier.
//
// Each method below uses bbolt's db.View (reads) or db.Update (writes) which
// internally handle tx lifecycle. The single exception is that returned
// slices from Get are copied — bbolt's underlying byte slice becomes invalid
// once db.View returns, so we MUST copy before exposing it.
type autoBucket struct {
	db   *bolt.DB
	name string
}

func (a autoBucket) Get(key []byte) []byte {
	var out []byte

	_ = a.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(a.name))
		if b == nil {
			return nil
		}

		v := b.Get(key)
		if v == nil {
			return nil
		}

		out = make([]byte, len(v))
		copy(out, v)

		return nil
	})

	return out
}

func (a autoBucket) Put(key, value []byte) error {
	err := a.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(a.name))
		if b == nil {
			return fmt.Errorf("bucket %q: %w", a.name, ErrBucketNotFound)
		}

		return b.Put(key, value)
	})
	if err != nil {
		return fmt.Errorf("bbolt put: %w", err)
	}

	return nil
}

func (a autoBucket) Delete(key []byte) error {
	err := a.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(a.name))
		if b == nil {
			return fmt.Errorf("bucket %q: %w", a.name, ErrBucketNotFound)
		}

		return b.Delete(key)
	})
	if err != nil {
		return fmt.Errorf("bbolt delete: %w", err)
	}

	return nil
}

func (a autoBucket) ForEach(fn func(k, v []byte) error) error {
	err := a.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(a.name))
		if b == nil {
			return fmt.Errorf("bucket %q: %w", a.name, ErrBucketNotFound)
		}

		return b.ForEach(fn)
	})
	if err != nil {
		return fmt.Errorf("bbolt foreach: %w", err)
	}

	return nil
}

// Cursor on autoBucket is intentionally unsupported — a cursor's lifetime is
// tied to a transaction, and autoBucket has no long-lived tx to bind to.
// Callers needing a cursor MUST wrap in WithTx / WithReadTx.
func (a autoBucket) Cursor() Cursor { return emptyCursor{} }

// boltCursor adapts *bolt.Cursor to the Cursor interface.
type boltCursor struct {
	c *bolt.Cursor
}

func (c boltCursor) First() ([]byte, []byte)             { return c.c.First() }
func (c boltCursor) Last() ([]byte, []byte)              { return c.c.Last() }
func (c boltCursor) Next() ([]byte, []byte)              { return c.c.Next() }
func (c boltCursor) Prev() ([]byte, []byte)              { return c.c.Prev() }
func (c boltCursor) Seek(prefix []byte) ([]byte, []byte) { return c.c.Seek(prefix) }

// emptyCursor is returned when a cursor cannot be produced (missing bucket
// or autoBucket). All positioning methods return (nil, nil).
type emptyCursor struct{}

func (emptyCursor) First() ([]byte, []byte)      { return nil, nil }
func (emptyCursor) Last() ([]byte, []byte)       { return nil, nil }
func (emptyCursor) Next() ([]byte, []byte)       { return nil, nil }
func (emptyCursor) Prev() ([]byte, []byte)       { return nil, nil }
func (emptyCursor) Seek([]byte) ([]byte, []byte) { return nil, nil }
