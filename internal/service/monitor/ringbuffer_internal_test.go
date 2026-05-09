package monitor

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func ev(method string) domain.ClientEvent {
	return domain.ClientEvent{Method: method, Timestamp: time.Now()}
}

func methods(evs []domain.ClientEvent) []string {
	out := make([]string, len(evs))
	for i, e := range evs {
		out[i] = e.Method
	}

	return out
}

func TestRingBuffer_EmptyInitially(t *testing.T) {
	t.Parallel()

	b := newEventRingBuffer(3)
	assert.Equal(t, 0, b.Len())
	assert.Empty(t, b.Snapshot())
}

func TestRingBuffer_FillsWithoutWrap(t *testing.T) {
	t.Parallel()

	b := newEventRingBuffer(3)
	b.Push(ev("a"))
	b.Push(ev("b"))

	assert.Equal(t, 2, b.Len())
	assert.Equal(t, []string{"a", "b"}, methods(b.Snapshot()))
}

func TestRingBuffer_ExactlyFull(t *testing.T) {
	t.Parallel()

	b := newEventRingBuffer(3)
	b.Push(ev("a"))
	b.Push(ev("b"))
	b.Push(ev("c"))

	assert.Equal(t, 3, b.Len())
	assert.Equal(t, []string{"a", "b", "c"}, methods(b.Snapshot()))
}

func TestRingBuffer_WrapsAndKeepsChronological(t *testing.T) {
	t.Parallel()

	b := newEventRingBuffer(3)
	b.Push(ev("a"))
	b.Push(ev("b"))
	b.Push(ev("c"))
	b.Push(ev("d")) // evicts "a"

	assert.Equal(t, 3, b.Len())
	assert.Equal(t, []string{"b", "c", "d"}, methods(b.Snapshot()),
		"after wrap, Snapshot must return oldest-first chronological order")
}

func TestRingBuffer_MultipleWraps(t *testing.T) {
	t.Parallel()

	b := newEventRingBuffer(3)
	for _, m := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		b.Push(ev(m))
	}

	// Last 3 pushed: e, f, g
	assert.Equal(t, []string{"e", "f", "g"}, methods(b.Snapshot()))
}

func TestRingBuffer_Capacity1(t *testing.T) {
	t.Parallel()

	b := newEventRingBuffer(1)
	b.Push(ev("a"))
	b.Push(ev("b"))
	b.Push(ev("c"))

	assert.Equal(t, []string{"c"}, methods(b.Snapshot()))
}

func TestRingBuffer_MinCapacityEnforced(t *testing.T) {
	t.Parallel()

	// capacity 0 or negative should be clamped to 1 to avoid divide-by-zero.
	b := newEventRingBuffer(0)
	b.Push(ev("x"))
	assert.Equal(t, []string{"x"}, methods(b.Snapshot()))
}

func TestRingBuffer_SnapshotIsIndependent(t *testing.T) {
	t.Parallel()

	// Mutating the snapshot must not affect the buffer.
	b := newEventRingBuffer(3)
	b.Push(ev("a"))
	b.Push(ev("b"))

	snap := b.Snapshot()
	snap[0].Method = "mutated"

	again := b.Snapshot()
	assert.Equal(t, "a", again[0].Method, "buffer must not be affected by caller mutating snapshot")
}

func TestRingBuffer_ConcurrentPush(t *testing.T) {
	t.Parallel()

	b := newEventRingBuffer(100)

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			for range 50 {
				b.Push(ev("x"))
			}
		})
	}

	wg.Wait()

	// 500 pushes, capacity 100 → buffer is full, length = capacity
	assert.Equal(t, 100, b.Len())
	assert.Len(t, b.Snapshot(), 100)
}
