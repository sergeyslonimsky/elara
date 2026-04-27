package webhook

import (
	"sync"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type deliveryRingBuffer struct {
	mu       sync.Mutex
	capacity int
	items    []domain.DeliveryAttempt
	head     int
	filled   bool
}

func newDeliveryRingBuffer(capacity int) *deliveryRingBuffer {
	if capacity < 1 {
		capacity = 1
	}

	return &deliveryRingBuffer{
		capacity: capacity,
		items:    make([]domain.DeliveryAttempt, capacity),
	}
}

func (r *deliveryRingBuffer) Push(a domain.DeliveryAttempt) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.items[r.head] = a
	r.head = (r.head + 1) % r.capacity

	if r.head == 0 {
		r.filled = true
	}
}

func (r *deliveryRingBuffer) Snapshot() []domain.DeliveryAttempt {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.filled {
		out := make([]domain.DeliveryAttempt, r.head)
		copy(out, r.items[:r.head])

		return out
	}

	out := make([]domain.DeliveryAttempt, r.capacity)
	copy(out, r.items[r.head:])
	copy(out[r.capacity-r.head:], r.items[:r.head])

	return out
}
