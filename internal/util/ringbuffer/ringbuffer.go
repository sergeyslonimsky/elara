package ringbuffer

import "sync"

// Buffer is a bounded ring buffer safe for concurrent use. When full, Push
// overwrites the oldest entry.
type Buffer[T any] struct {
	mu       sync.Mutex
	capacity int
	items    []T
	head     int
	filled   bool
}

// New creates a Buffer with the given capacity (minimum 1).
func New[T any](capacity int) *Buffer[T] {
	if capacity < 1 {
		capacity = 1
	}

	return &Buffer[T]{
		capacity: capacity,
		items:    make([]T, capacity),
	}
}

// Push adds v to the buffer, evicting the oldest entry when full.
func (r *Buffer[T]) Push(v T) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.items[r.head] = v
	r.head = (r.head + 1) % r.capacity

	if r.head == 0 {
		r.filled = true
	}
}

// Snapshot returns a copy of the buffer contents in oldest-first order.
func (r *Buffer[T]) Snapshot() []T {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.filled {
		out := make([]T, r.head)
		copy(out, r.items[:r.head])

		return out
	}

	out := make([]T, r.capacity)
	copy(out, r.items[r.head:])
	copy(out[r.capacity-r.head:], r.items[:r.head])

	return out
}

// Len returns the number of items currently stored.
func (r *Buffer[T]) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.filled {
		return r.capacity
	}

	return r.head
}
