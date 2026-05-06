package monitor

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestPublisher_Subscribe(t *testing.T) {
	t.Parallel()

	t.Run("returns closed channel after shutdown", func(t *testing.T) {
		t.Parallel()
		p := newPublisher(1)
		p.shutdown()

		ch, _ := p.subscribe()
		_, ok := <-ch
		assert.False(t, ok, "should return closed channel after shutdown")
	})

	t.Run("assigns unique IDs", func(t *testing.T) {
		t.Parallel()
		p := newPublisher(1)

		_, _ = p.subscribe()
		_, _ = p.subscribe()

		assert.Equal(t, 2, p.nextID)
		assert.Len(t, p.subs, 2)
	})
}

func TestPublisher_Publish(t *testing.T) {
	t.Parallel()

	t.Run("fans out to all subscribers", func(t *testing.T) {
		t.Parallel()
		p := newPublisher(1)
		ch1, _ := p.subscribe()
		ch2, _ := p.subscribe()

		event := domain.ClientChange{Kind: domain.ClientConnected}
		p.publish(event)

		select {
		case ev := <-ch1:
			assert.Equal(t, event, ev)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("sub1 did not receive event")
		}

		select {
		case ev := <-ch2:
			assert.Equal(t, event, ev)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("sub2 did not receive event")
		}
	})

	t.Run("does not block on slow subscriber", func(t *testing.T) {
		t.Parallel()
		p := newPublisher(1)
		ch1, _ := p.subscribe() // capacity 1

		// Fill the buffer
		p.publish(domain.ClientChange{Kind: domain.ClientConnected})

		// This should drop and NOT block
		done := make(chan struct{})
		go func() {
			p.publish(domain.ClientChange{Kind: domain.ClientDisconnected})
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("publish blocked on full subscriber buffer")
		}

		// Verify ch1 still has the first event
		select {
		case ev := <-ch1:
			assert.Equal(t, domain.ClientConnected, ev.Kind)
		default:
			t.Fatal("sub1 should have received the first event")
		}
	})
}

func TestPublisher_Cleanup(t *testing.T) {
	t.Parallel()

	t.Run("unsubscribes and closes channel", func(t *testing.T) {
		t.Parallel()
		p := newPublisher(1)
		ch, cancel := p.subscribe()

		assert.Len(t, p.subs, 1)
		cancel()

		assert.Empty(t, p.subs)
		_, ok := <-ch
		assert.False(t, ok, "channel should be closed after unsubscribe")
	})

	t.Run("idempotent", func(t *testing.T) {
		t.Parallel()
		p := newPublisher(1)
		_, cancel := p.subscribe()

		cancel()
		cancel() // should not panic
	})
}

func TestPublisher_Shutdown(t *testing.T) {
	t.Parallel()

	p := newPublisher(1)
	ch1, _ := p.subscribe()
	ch2, _ := p.subscribe()

	p.shutdown()

	assert.True(t, p.stopped)
	assert.Empty(t, p.subs)

	_, ok1 := <-ch1
	assert.False(t, ok1)

	_, ok2 := <-ch2
	assert.False(t, ok2)
}

func TestPublisher_Concurrent(t *testing.T) {
	t.Parallel()

	p := newPublisher(100)
	var wg sync.WaitGroup

	// Concurrent subscribers
	for range 10 {
		wg.Go(func() {
			ch, cancel := p.subscribe()
			time.Sleep(10 * time.Millisecond)
			cancel()
			_ = ch
		})
	}

	// Concurrent publishers
	for range 10 {
		wg.Go(func() {
			for range 50 {
				p.publish(domain.ClientChange{})
			}
		})
	}

	wg.Wait()
}
