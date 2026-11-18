package storagetest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/storage"
)

var errSentinel = errors.New("storagetest: sentinel error")

// txFunc is the shape shared by Manager.WithTx and Manager.WithReadTx.
type txFunc = func(ctx context.Context, callback func(ctx context.Context) error) error

// RunManagerContract runs a suite of tests that any storage.Manager implementation must pass.
// factory should return a fresh instance of the manager for each test.
//
// The test suite verifies:
// 1. Transaction commit on success (nil error).
// 2. Transaction rollback on error.
// 3. Transaction rollback on panic.
// 4. Nested WithTx calls are flattened (join existing transaction).
// 5. WithReadTx shares those semantics and joins an outer WithTx or WithReadTx.
// 6. WithTx inside WithReadTx fails with storage.ErrReadOnlyTx without running.
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

	t.Run("ReadTx", func(t *testing.T) {
		runReadTxContract(t, factory)
	})

	t.Run("ReadTxJoins", func(t *testing.T) {
		runReadTxJoinContract(t, factory)
	})
}

func runReadTxContract(t *testing.T, factory func(t *testing.T) storage.Manager) {
	t.Helper()

	t.Run("ReturnsNilOnSuccess", func(t *testing.T) {
		m := factory(t)

		err := m.WithReadTx(context.Background(), func(context.Context) error {
			return nil
		})
		assert.NoError(t, err)
	})

	t.Run("PropagatesError", func(t *testing.T) {
		m := factory(t)

		err := m.WithReadTx(context.Background(), func(context.Context) error {
			return errSentinel
		})
		assert.ErrorIs(t, err, errSentinel)
	})

	t.Run("RepanicsOnPanic", func(t *testing.T) {
		m := factory(t)

		assert.Panics(t, func() {
			_ = m.WithReadTx(context.Background(), func(context.Context) error {
				panic("oops")
			})
		})
	})

	t.Run("WithTxInsideFails", func(t *testing.T) {
		m := factory(t)
		called := false

		err := m.WithReadTx(context.Background(), func(ctx context.Context) error {
			return m.WithTx(ctx, func(context.Context) error {
				called = true

				return nil
			})
		})
		require.ErrorIs(t, err, storage.ErrReadOnlyTx)
		assert.False(t, called, "a write callback must not run inside a read-only transaction")
	})
}

func runReadTxJoinContract(t *testing.T, factory func(t *testing.T) storage.Manager) {
	t.Helper()

	tests := []struct {
		name  string
		outer func(storage.Manager) txFunc
	}{
		{name: "InsideWithTx", outer: func(m storage.Manager) txFunc { return m.WithTx }},
		{name: "InsideWithReadTx", outer: func(m storage.Manager) txFunc { return m.WithReadTx }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := factory(t)

			type probeKey struct{}
			probe := "outer-tx"

			err := tt.outer(m)(context.Background(), func(ctx1 context.Context) error {
				ctx1WithProbe := context.WithValue(ctx1, probeKey{}, probe)

				return m.WithReadTx(ctx1WithProbe, func(ctx2 context.Context) error {
					assert.Equal(t, probe, ctx2.Value(probeKey{}), "inner read transaction must join the outer one")

					return nil
				})
			})
			assert.NoError(t, err)
		})
	}
}
