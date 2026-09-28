package lease_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/usecase/lease"
)

// sweepSignal reports each sweep the expirer performs, so the test waits on the
// worker's own progress instead of on a sleep.
func sweepSignal(m mocks, err error) chan struct{} {
	swept := make(chan struct{}, 16)

	m.leases.EXPECT().
		ExpiredBefore(gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes().
		DoAndReturn(func(_ context.Context, _ time.Time, _ int) ([]int64, error) {
			select {
			case swept <- struct{}{}:
			default:
			}

			return nil, err
		})

	return swept
}

func TestExpirer_Run(t *testing.T) {
	t.Parallel()

	t.Run("sweeps until the context is cancelled", func(t *testing.T) {
		t.Parallel()

		svc, m := setupService(t, lease.Config{})
		swept := sweepSignal(m, nil)
		expirer := lease.NewExpirer(svc, time.Millisecond, 8)

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)

		go func() { done <- expirer.Run(ctx) }()

		for range 2 {
			select {
			case <-swept:
			case <-time.After(2 * time.Second):
				t.Fatal("expected the expirer to sweep")
			}
		}

		cancel()

		select {
		case err := <-done:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(2 * time.Second):
			t.Fatal("expected Run to return once the context was cancelled")
		}
	})

	t.Run("a failing sweep does not stop the loop", func(t *testing.T) {
		t.Parallel()

		// A lease that cannot be revoked right now — a locked namespace, say —
		// must not take the worker down with it; the next tick retries.
		svc, m := setupService(t, lease.Config{})
		swept := sweepSignal(m, errors.New("index unavailable"))
		expirer := lease.NewExpirer(svc, time.Millisecond, 8)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		go func() { _ = expirer.Run(ctx) }()

		for range 3 {
			select {
			case <-swept:
			case <-time.After(2 * time.Second):
				t.Fatal("expected the expirer to keep sweeping after a failure")
			}
		}
	})
}

func TestNewExpirer_RejectsUnusableSettings(t *testing.T) {
	t.Parallel()

	// time.NewTicker panics on a non-positive interval, so a Service assembled by
	// hand — a test, a tool — must not be able to boot a sweep that dies on its
	// first tick.
	svc, m := setupService(t, lease.Config{})
	sweepSignal(m, nil)

	expirer := lease.NewExpirer(svc, 0, 0)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() { done <- expirer.Run(ctx) }()

	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("expected Run to return once the context was cancelled")
	}
}
