package watch

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

const defaultBufferSize = 100

type subscription struct {
	pathPrefix string
	namespace  string
	events     chan domain.WatchEvent
}

type Publisher struct {
	subscriptions map[string]*subscription
	mu            sync.RWMutex
	nextID        int
}

func NewPublisher() *Publisher {
	return &Publisher{
		subscriptions: make(map[string]*subscription),
	}
}

// Subscribe registers a new subscription filtered by pathPrefix+namespace and
// returns an event channel and an idempotent cleanup function.
//
// Lifecycle contract:
//   - The caller MUST invoke cleanup() when done, typically via defer. Failing
//     to do so leaks the subscription (channel + map entry) until Publisher
//     shutdown.
//   - ctx is NOT used to auto-cancel the subscription. The cleanup function is
//     the only way to release resources. The ctx parameter is preserved for
//     future use and for compatibility with cancellable callers.
//   - The returned channel is closed by cleanup() (or by Shutdown()). Readers
//     must handle a closed channel.
//   - cleanup() is safe to call multiple times (sync.Once-guarded).
func (p *Publisher) Subscribe(
	_ context.Context,
	pathPrefix, namespace string,
) (<-chan domain.WatchEvent, func()) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.nextID++
	id := strconv.Itoa(p.nextID)

	sub := &subscription{
		pathPrefix: pathPrefix,
		namespace:  namespace,
		events:     make(chan domain.WatchEvent, defaultBufferSize),
	}

	p.subscriptions[id] = sub

	var cleanupOnce sync.Once

	cleanup := func() {
		cleanupOnce.Do(func() {
			p.mu.Lock()
			defer p.mu.Unlock()

			if s, ok := p.subscriptions[id]; ok {
				close(s.events)
				delete(p.subscriptions, id)
			}
		})
	}

	return sub.events, cleanup
}

func (p *Publisher) NotifyCreated(_ context.Context, cfg *domain.Config) {
	p.notify(domain.WatchEvent{
		Type:      domain.EventTypeCreated,
		Path:      cfg.Path,
		Namespace: cfg.Namespace,
		Revision:  cfg.Revision,
		Config:    cfg,
		Timestamp: time.Now(),
	})
}

func (p *Publisher) NotifyUpdated(_ context.Context, cfg *domain.Config) {
	p.notify(domain.WatchEvent{
		Type:      domain.EventTypeUpdated,
		Path:      cfg.Path,
		Namespace: cfg.Namespace,
		Revision:  cfg.Revision,
		Config:    cfg,
		Timestamp: time.Now(),
	})
}

func (p *Publisher) NotifyDeleted(_ context.Context, path, namespace string, revision int64) {
	p.notify(domain.WatchEvent{
		Type:      domain.EventTypeDeleted,
		Path:      path,
		Namespace: namespace,
		Revision:  revision,
		Timestamp: time.Now(),
	})
}

func (p *Publisher) Shutdown() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id, sub := range p.subscriptions {
		close(sub.events)
		delete(p.subscriptions, id)
	}
}

func (p *Publisher) notify(event domain.WatchEvent) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, sub := range p.subscriptions {
		if !matches(sub, event) {
			continue
		}

		select {
		case sub.events <- event:
		default:
			slog.Warn("watch event dropped: subscriber buffer full",
				"path", event.Path,
				"namespace", event.Namespace,
				"type", event.Type.String(),
			)
		}
	}
}

func matches(sub *subscription, event domain.WatchEvent) bool {
	if sub.namespace != "" && sub.namespace != event.Namespace {
		return false
	}

	if sub.pathPrefix != "" && !strings.HasPrefix(event.Path, sub.pathPrefix) {
		return false
	}

	return true
}
