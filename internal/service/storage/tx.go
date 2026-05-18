package storage

//go:generate mockgen -destination=mocks/tx_mock.go -package=storage_mock -source=tx.go

import (
	"context"
)

// Tx represents a database transaction.
type Tx interface {
	// Bucket returns a bucket by name.
	Bucket(name []byte) Bucket
}

// Bucket represents a key-value bucket within a transaction.
type Bucket interface {
	// Get retrieves the value for a key.
	Get(key []byte) []byte
	// Put sets the value for a key.
	Put(key, value []byte) error
	// Delete removes a key.
	Delete(key []byte) error
	// ForEach executes a function for each key-value pair in the bucket.
	ForEach(fn func(k, v []byte) error) error
}

// TxManager manages database transactions.
type TxManager interface {
	// Read executes a read-only transaction.
	Read(ctx context.Context, fn func(Tx) error) error
	// Write executes a read-write transaction.
	Write(ctx context.Context, fn func(Tx) error) error
}
