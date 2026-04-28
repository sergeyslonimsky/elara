package webhook

import (
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/util/ringbuffer"
)

type deliveryRingBuffer struct {
	buf *ringbuffer.Buffer[domain.DeliveryAttempt]
}

func newDeliveryRingBuffer(capacity int) *deliveryRingBuffer {
	return &deliveryRingBuffer{buf: ringbuffer.New[domain.DeliveryAttempt](capacity)}
}

func (r *deliveryRingBuffer) Push(a domain.DeliveryAttempt) { r.buf.Push(a) }

func (r *deliveryRingBuffer) Snapshot() []domain.DeliveryAttempt { return r.buf.Snapshot() }
