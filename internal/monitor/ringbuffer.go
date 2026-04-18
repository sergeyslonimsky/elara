package monitor

import (
	"sync"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// eventRingBuffer is a bounded ring buffer of ClientEvents. When full, pushing
// a new event overwrites the oldest. Safe for concurrent use.
type eventRingBuffer struct {
	mu       sync.Mutex
	capacity int
	items    []domain.ClientEvent
	head     int  // index where the next Push will write
	filled   bool // true once capacity items have been pushed (head wraps back to 0)
}

func newEventRingBuffer(capacity int) *eventRingBuffer {
	if capacity < 1 {
		capacity = 1
	}

	return &eventRingBuffer{
		capacity: capacity,
		items:    make([]domain.ClientEvent, capacity),
	}
}

// Push appends an event. If the buffer is full, the oldest entry is overwritten.
func (r *eventRingBuffer) Push(ev domain.ClientEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.items[r.head] = ev
	r.head = (r.head + 1) % r.capacity

	// Invariant: filled is set the first time head wraps to 0, meaning all
	// capacity slots have been written at least once.
	if r.head == 0 {
		r.filled = true
	}
}

// Snapshot returns a copy of the buffer contents in chronological order
// (oldest first). The returned slice is safe to retain; subsequent Pushes
// do not affect it.
func (r *eventRingBuffer) Snapshot() []domain.ClientEvent {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.filled {
		// Buffer not yet wrapped — events are at [0, head).
		out := make([]domain.ClientEvent, r.head)
		copy(out, r.items[:r.head])

		return out
	}

	// Wrapped: oldest is at head, newest at head-1 (mod capacity).
	out := make([]domain.ClientEvent, r.capacity)
	copy(out, r.items[r.head:])
	copy(out[r.capacity-r.head:], r.items[:r.head])

	return out
}

// Len returns the current number of events stored (≤ capacity).
func (r *eventRingBuffer) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.filled {
		return r.capacity
	}

	return r.head
}
