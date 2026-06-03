package bbolt

import (
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// Querier is the minimal interface implemented by both transaction-backed and
// auto-tx Querier types. It is the bbolt analogue of core/sql.Querier:
// generic executors (Get, List, Put, Delete in executor.go) accept a Querier
// so they work uniformly inside or outside a transaction.
//
// Repositories should depend only on this interface, never on *bolt.Tx or
// *bolt.DB directly.
type Querier interface {
	// Bucket returns a handle to the named top-level bucket. The handle is
	// valid only for the lifetime of the underlying transaction (for
	// txQuerier) or for the duration of a single subsequent operation (for
	// autoQuerier).
	//
	// If the bucket does not exist, returned Bucket operations error with
	// ErrBucketNotFound.
	Bucket(name string) Bucket
}

// txQuerier is the Querier backed by an explicit *bolt.Tx (joined via WithTx
// or WithReadTx). All Bucket operations execute against that same tx, giving
// the caller multi-op atomicity.
type txQuerier struct {
	tx *bolt.Tx
}

// Bucket returns a tx-bound Bucket handle. The handle is invalidated when
// the underlying transaction commits or rolls back.
func (q txQuerier) Bucket(name string) Bucket {
	b := q.tx.Bucket([]byte(name))
	if b == nil {
		return missingBucket{name: name}
	}

	return txBucket{b: b}
}

// autoQuerier is the Querier returned when ctx carries no transaction. Each
// Bucket operation opens its own short-lived bbolt transaction. This mirrors
// how *sql.DB acts as a Querier in core/sql — operations work, but they are
// NOT atomic across calls.
type autoQuerier struct {
	db *bolt.DB
}

// Bucket returns a Bucket handle that opens a fresh tx per operation.
func (q autoQuerier) Bucket(name string) Bucket {
	return autoBucket{db: q.db, name: name}
}

// missingBucket is returned when a tx-backed Querier is asked for a bucket
// that does not exist. All operations on it return ErrBucketNotFound — this
// preserves the contract that Bucket() never returns nil.
type missingBucket struct {
	name string
}

func (m missingBucket) Get(_ []byte) []byte { return nil }

func (m missingBucket) Put(_, _ []byte) error {
	return fmt.Errorf("bucket %q: %w", m.name, ErrBucketNotFound)
}

func (m missingBucket) Delete(_ []byte) error {
	return fmt.Errorf("bucket %q: %w", m.name, ErrBucketNotFound)
}

func (m missingBucket) ForEach(_ func(k, v []byte) error) error {
	return fmt.Errorf("bucket %q: %w", m.name, ErrBucketNotFound)
}

func (m missingBucket) Cursor() Cursor { return emptyCursor{} }
