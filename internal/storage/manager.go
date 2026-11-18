package storage

import (
	"context"
)

//go:generate mockgen -destination=mocks/manager_mock.go -package=storage_mock -source=manager.go

// Manager defines the interface for transaction management.
// It is designed to be backend-agnostic and follows the pattern
// where the transaction is propagated implicitly via context.
type Manager interface {
	// WithTx executes the given callback within a transaction.
	// The provided context in the callback carries the transaction handle.
	// If the callback returns an error, the transaction is rolled back.
	// If the callback panics, the transaction is rolled back (best-effort)
	// and the panic is re-raised.
	//
	// Called inside WithReadTx it fails with ErrReadOnlyTx: a read-only
	// boundary cannot be upgraded.
	WithTx(ctx context.Context, callback func(ctx context.Context) error) error

	// WithReadTx executes the given callback within a read-only transaction,
	// so every read inside it observes one consistent snapshot. Reads made
	// without it may each see a different committed state.
	//
	// Nested inside WithTx or WithReadTx it joins the outer transaction. Use
	// it only for flows that never write; see WithTx for what happens if one
	// does. Same rollback and panic semantics as WithTx.
	WithReadTx(ctx context.Context, callback func(ctx context.Context) error) error
}
