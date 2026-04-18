package monitor

import (
	"log/slog"
	"strconv"
	"sync"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// publisher is a fan-out for ClientChange events. It is internal to the monitor
// package — Registry exposes only Subscribe/Shutdown.
//
// Slow subscribers do NOT block the publisher: when a subscriber's buffered
// channel is full, the event is dropped with a warn log. This mirrors the
// design of adapter/watch/publisher.go.
type publisher struct {
	bufferSize int

	mu      sync.RWMutex
	subs    map[string]*subscription
	nextID  int
	stopped bool
}

type subscription struct {
	id  string
	ch  chan domain.ClientChange
	ack chan struct{} // closed once cleanup has run, makes cleanup idempotent
}

func newPublisher(bufferSize int) *publisher {
	if bufferSize < 1 {
		bufferSize = 1
	}

	return &publisher{
		bufferSize: bufferSize,
		subs:       make(map[string]*subscription),
	}
}

func (p *publisher) subscribe() (<-chan domain.ClientChange, func()) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped {
		// Return an already-closed channel so receivers don't hang.
		closed := make(chan domain.ClientChange)
		close(closed)

		return closed, func() {}
	}

	p.nextID++
	id := strconv.Itoa(p.nextID)

	sub := &subscription{
		id:  id,
		ch:  make(chan domain.ClientChange, p.bufferSize),
		ack: make(chan struct{}),
	}
	p.subs[id] = sub

	cleanup := func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		select {
		case <-sub.ack:
			// already cleaned up — idempotent
			return
		default:
		}

		close(sub.ack)

		if _, ok := p.subs[id]; ok {
			delete(p.subs, id)
			close(sub.ch)
		}
	}

	return sub.ch, cleanup
}

// publish fans the change out to all subscribers. Drops on full subscriber
// buffer with a warn log.
func (p *publisher) publish(c domain.ClientChange) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, sub := range p.subs {
		select {
		case sub.ch <- c:
		default:
			slog.Warn("monitor: client change event dropped: subscriber buffer full",
				"sub_id", sub.id,
				"kind", c.Kind,
			)
		}
	}
}

// shutdown closes all active subscriptions and prevents future ones.
func (p *publisher) shutdown() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stopped = true

	for id, sub := range p.subs {
		select {
		case <-sub.ack:
		default:
			close(sub.ack)
			close(sub.ch)
		}

		delete(p.subs, id)
	}
}
