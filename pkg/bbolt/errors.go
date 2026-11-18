package bbolt

import "errors"

// ErrNotFound is returned by generic helpers (Get[T]) when the requested key
// is not present in the bucket. Callers typically wrap this with a
// domain-specific not-found error.
var ErrNotFound = errors.New("bbolt: not found")

// ErrBucketNotFound is returned when a Querier is asked to operate on a
// bucket name that does not exist in the underlying database. Bucket
// creation is the responsibility of the application bootstrap path — this
// package does not auto-create buckets on access.
var ErrBucketNotFound = errors.New("bbolt: bucket not found")

// ErrTxReadOnly is returned by WithTx when ctx already carries a read-only
// transaction. Opening a writer there would put a read-only and a read-write
// transaction in one goroutine, which bbolt documents as a deadlock: the
// writer may need to remap the file while the reader pins the old mapping.
var ErrTxReadOnly = errors.New("bbolt: write requested inside a read-only transaction")
