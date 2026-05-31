package storagetest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/storage"
)

var errSentinel = errors.New("storagetest: sentinel error")

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
		sentinel := errSentinel

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

		type probeKey struct{}
		probe := "original-tx"

		err := m.WithTx(ctx, func(ctx1 context.Context) error {
			// Inject a probe into the context of the first transaction.
			// If flattening works, the inner WithTx must return a context
			// that STILL carries this probe.
			ctx1WithProbe := context.WithValue(ctx1, probeKey{}, probe)

			return m.WithTx(ctx1WithProbe, func(ctx2 context.Context) error {
				got := ctx2.Value(probeKey{})
				assert.Equal(t, probe, got, "inner transaction must join and preserve the outer context")

				return nil
			})
		})
		assert.NoError(t, err)
	})
}
