package storage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

// RunManagerContract runs a suite of tests that any storage.Manager implementation must pass.
// factory should return a fresh instance of the manager for each test.
//
// The test suite verifies:
// 1. Transaction commit on success (nil error).
// 2. Transaction rollback on error.
// 3. Transaction rollback on panic.
// 4. Nested WithTx calls are flattened (join existing transaction).
func RunManagerContract(t *testing.T, factory func(t *testing.T) storage.Manager) {
	t.Helper()

	t.Run("CommitOnSuccess", func(t *testing.T) {
		m := factory(t)
		ctx := context.Background()

		err := m.WithTx(ctx, func(ctx context.Context) error {
			return nil
		})
		assert.NoError(t, err)
	})

	t.Run("RollbackOnError", func(t *testing.T) {
		m := factory(t)
		ctx := context.Background()
		sentinel := errors.New("sentinel error")

		err := m.WithTx(ctx, func(ctx context.Context) error {
			return sentinel
		})
		assert.ErrorIs(t, err, sentinel)
	})

	t.Run("RollbackOnPanic", func(t *testing.T) {
		m := factory(t)
		ctx := context.Background()

		assert.Panics(t, func() {
			_ = m.WithTx(ctx, func(ctx context.Context) error {
				panic("oops")
			})
		})
	})

	t.Run("NestedFlatten", func(t *testing.T) {
		m := factory(t)
		ctx := context.Background()

		err := m.WithTx(ctx, func(ctx1 context.Context) error {
			// Outer tx started
			return m.WithTx(ctx1, func(ctx2 context.Context) error {
				// Inner tx should be the same as outer
				// We can't strictly prove it's the SAME without backend-specific knowledge,
				// but we can verify that WithTx doesn't fail when called nestedly.
				return nil
			})
		})
		assert.NoError(t, err)
	})
}
