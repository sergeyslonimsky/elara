package bbolt

import (
	"context"
	"fmt"

	"go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/storage"
)

var _ storage.Manager = (*Manager)(nil)

// Manager implements storage.Manager for bbolt.
type Manager struct {
	db *bbolt.DB
}

// NewManager creates a new bbolt manager.
func NewManager(db *bbolt.DB) *Manager {
	return &Manager{db: db}
}

// WithTx executes the callback within a bbolt transaction.
// If a transaction already exists in the context, it joins it (flattening).
func (m *Manager) WithTx(ctx context.Context, callback func(ctx context.Context) error) error {
	// 1. Check if transaction already exists in context (flattening)
	if _, ok := txFromCtx(ctx); ok {
		return callback(ctx)
	}

	// 2. Start new transaction
	tx, err := m.db.Begin(true)
	if err != nil {
		return fmt.Errorf("bbolt: begin tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p) // re-panic
		}
	}()

	// 3. Put tx into context and execute callback
	err = callback(withTx(ctx, tx))
	// 4. Commit or rollback
	if err != nil {
		_ = tx.Rollback()

		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("bbolt: commit tx: %w", err)
	}

	return nil
}

// View executes a read-only callback. It joins an existing transaction if
// present in context, or starts a new one.
func (m *Manager) View(ctx context.Context, fn func(*bbolt.Tx) error) error {
	if tx, ok := txFromCtx(ctx); ok {
		return fn(tx)
	}

	if err := m.db.View(func(tx *bbolt.Tx) error {
		return fn(tx)
	}); err != nil {
		return fmt.Errorf("bbolt: view tx: %w", err)
	}

	return nil
}

// Update executes a read-write callback. It joins an existing transaction if
// present in context, or starts a new one via WithTx.
func (m *Manager) Update(ctx context.Context, fn func(*bbolt.Tx) error) error {
	if tx, ok := txFromCtx(ctx); ok {
		return fn(tx)
	}

	return m.WithTx(ctx, func(ctx context.Context) error {
		tx, _ := txFromCtx(ctx)

		return fn(tx)
	})
}
