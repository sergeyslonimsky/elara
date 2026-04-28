package monitor

import (
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/util/ringbuffer"
)

type eventRingBuffer struct {
	buf *ringbuffer.Buffer[domain.ClientEvent]
}

func newEventRingBuffer(capacity int) *eventRingBuffer {
	return &eventRingBuffer{buf: ringbuffer.New[domain.ClientEvent](capacity)}
}

func (r *eventRingBuffer) Push(ev domain.ClientEvent) { r.buf.Push(ev) }

func (r *eventRingBuffer) Snapshot() []domain.ClientEvent { return r.buf.Snapshot() }

func (r *eventRingBuffer) Len() int { return r.buf.Len() }
