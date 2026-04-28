package ringbuffer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/util/ringbuffer"
)

func TestBuffer_PushAndSnapshot_BelowCapacity(t *testing.T) {
	t.Parallel()

	buf := ringbuffer.New[int](5)

	buf.Push(1)
	buf.Push(2)
	buf.Push(3)

	assert.Equal(t, 3, buf.Len())
	assert.Equal(t, []int{1, 2, 3}, buf.Snapshot())
}

func TestBuffer_PushAndSnapshot_ExactCapacity(t *testing.T) {
	t.Parallel()

	buf := ringbuffer.New[int](3)

	buf.Push(1)
	buf.Push(2)
	buf.Push(3)

	assert.Equal(t, 3, buf.Len())
	assert.Equal(t, []int{1, 2, 3}, buf.Snapshot())
}

func TestBuffer_OverflowEvictsOldest(t *testing.T) {
	t.Parallel()

	buf := ringbuffer.New[int](3)

	for i := 1; i <= 5; i++ {
		buf.Push(i)
	}

	assert.Equal(t, 3, buf.Len())
	// Oldest 2 were evicted; remaining in order: 3, 4, 5.
	assert.Equal(t, []int{3, 4, 5}, buf.Snapshot())
}

func TestBuffer_CapacityBelowOne_DefaultsToOne(t *testing.T) {
	t.Parallel()

	buf := ringbuffer.New[int](0)

	buf.Push(42)
	buf.Push(99)

	assert.Equal(t, 1, buf.Len())
	assert.Equal(t, []int{99}, buf.Snapshot())
}

func TestBuffer_Empty_SnapshotReturnsEmpty(t *testing.T) {
	t.Parallel()

	buf := ringbuffer.New[string](10)

	assert.Equal(t, 0, buf.Len())
	assert.Empty(t, buf.Snapshot())
}

func TestBuffer_SnapshotDoesNotMutateBuffer(t *testing.T) {
	t.Parallel()

	buf := ringbuffer.New[int](3)
	buf.Push(1)
	buf.Push(2)

	snap := buf.Snapshot()
	snap[0] = 999

	assert.Equal(t, []int{1, 2}, buf.Snapshot(), "mutating snapshot must not affect buffer")
}
