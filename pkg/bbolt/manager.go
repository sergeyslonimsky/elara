// Package bbolt is a transaction-orchestration layer over go.etcd.io/bbolt.
//
// It mirrors the API of github.com/sergeyslonimsky/core/sql so repositories
// written against this package can be re-pointed at a SQL backend (or vice
// versa) with no signature changes — they always call manager.GetQuerier(ctx)
// and operate against the returned Querier.
//
// The package has zero project-specific dependencies and is designed to be
// extracted into a shared core repository.
package bbolt

import (
	"context"
	"errors"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// txKey is the private context key used to thread an active transaction
// through callback chains inside WithTx / WithReadTx. Struct{} type avoids
// any collision with keys defined in other packages.
type txKey struct{}

// Manager is the transaction-orchestration interface used by repositories.
//
// Repositories should call GetQuerier(ctx) on every operation — the returned
// Querier is either the active transaction's Querier (when WithTx/WithReadTx
// wrapped the call chain) or an auto-tx Querier backed by the underlying *DB
// that opens a short-lived transaction per call.
type Manager interface {
	// WithTx runs callback inside a single read-write transaction.
	// The callback's ctx carries the transaction; subsequent GetQuerier(ctx)
	// calls return the transaction-backed Querier. On error, the transaction
	// is rolled back. On success, it is committed.
	//
	// Nested WithTx flattens: the inner call reuses the outer transaction.
	WithTx(ctx context.Context, callback func(context.Context) error) error

	// WithReadTx runs callback inside a single read-only transaction.
	// Same semantics as WithTx, but writes will fail with bbolt's tx
	// read-only error.
	//
	// Nested WithReadTx flattens. Nesting WithReadTx inside WithTx is allowed
	// (the outer read-write tx is reused).
	WithReadTx(ctx context.Context, callback func(context.Context) error) error

	// GetQuerier returns the active transaction's Querier if ctx was produced
	// by WithTx/WithReadTx; otherwise returns an auto-tx Querier that opens a
	// short-lived transaction per operation.
	//
	// IMPORTANT: when no transaction is in ctx, atomicity across multiple
	// GetQuerier(ctx) operations is NOT guaranteed — each call opens its own
	// short-lived tx. Callers requiring multi-op atomicity must wrap in
	// WithTx.
	GetQuerier(ctx context.Context) Querier
}

// DBManager is the default Manager implementation backed by a *bolt.DB.
type DBManager struct {
	db *bolt.DB
}

// NewManager wraps a *bolt.DB in a transaction Manager.
func NewManager(db *bolt.DB) *DBManager {
	return &DBManager{db: db}
}

// DB returns the underlying *bolt.DB. Intended for bootstrap code that needs
// raw access (bucket creation, backup, compaction). Repositories must not use
// this — they go through GetQuerier.
func (m *DBManager) DB() *bolt.DB {
	return m.db
}

// GetQuerier — see Manager.GetQuerier.
func (m *DBManager) GetQuerier(ctx context.Context) Querier {
	if tx, ok := ctx.Value(txKey{}).(*bolt.Tx); ok {
		return txQuerier{tx: tx}
	}

	return autoQuerier{db: m.db}
}

// WithTx — see Manager.WithTx.
func (m *DBManager) WithTx(ctx context.Context, callback func(context.Context) error) error {
	if existing, ok := ctx.Value(txKey{}).(*bolt.Tx); ok && existing.Writable() {
		return callback(ctx)
	}

	tx, err := m.db.Begin(true)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	return m.runTx(ctx, tx, callback)
}

// WithReadTx — see Manager.WithReadTx.
func (m *DBManager) WithReadTx(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*bolt.Tx); ok {
		return callback(ctx)
	}

	tx, err := m.db.Begin(false)
	if err != nil {
		return fmt.Errorf("begin read tx: %w", err)
	}

	return m.runTx(ctx, tx, callback)
}

// runTx is the shared body of WithTx / WithReadTx after a fresh tx has been
// opened. It handles panic-safety, runtime.Goexit-safety (testify's FailNow),
// rollback on error, and commit on success.
//
// Completion is tracked with `done` so the deferred rollback catches three
// exit paths: panic (recovered + re-raised below), runtime.Goexit (deferred
// runs but recover returns nil), and an early return that forgot to set
// done. Without this, a Goexit inside the callback leaves the tx dangling
// and the next DB.Close deadlocks waiting for the open tx.
//
// Rollback error from the deferred path is intentionally discarded — by then
// the original error (panic, Goexit, or callback error) is already in flight.
func (m *DBManager) runTx(
	ctx context.Context,
	tx *bolt.Tx,
	callback func(context.Context) error,
) (err error) {
	txCtx := context.WithValue(ctx, txKey{}, tx)

	done := false
	defer func() {
		if !done {
			_ = tx.Rollback()
		}
		if p := recover(); p != nil {
			panic(p)
		}
	}()

	if err = callback(txCtx); err != nil {
		rbErr := tx.Rollback()
		done = true

		return errors.Join(err, rbErr)
	}

	// Read-only transactions in bbolt must be released via Rollback (Commit
	// returns ErrTxNotWritable). Writable transactions go through Commit so
	// their changes hit disk.
	if tx.Writable() {
		if err = tx.Commit(); err != nil {
			done = true

			return fmt.Errorf("commit tx: %w", err)
		}
	} else {
		_ = tx.Rollback()
	}

	done = true

	return nil
}

// Compile-time check.
var _ Manager = (*DBManager)(nil)
