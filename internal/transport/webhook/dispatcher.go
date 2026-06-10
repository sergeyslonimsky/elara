package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/sergeyslonimsky/core/lifecycle"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

const (
	httpRequestTimeout      = 10 * time.Second
	deliveryHistorySize     = 50
	successStatusMin        = 200
	successStatusMax        = 300
	maxConcurrentDeliveries = 100
	jitterRange             = 5 // base = 4/5 of delay, window = 2/5 → ±20%
	jitterWindowFactor      = 2 // window = delay/jitterRange * jitterWindowFactor
)

//go:generate mockgen -destination=mocks/mock_dispatcher.go -package=webhook_mock . webhookLister,eventPublisher

type webhookLister interface {
	List(ctx context.Context) ([]*domain.Webhook, error)
}

type eventPublisher interface {
	Subscribe(ctx context.Context, pathPrefix, namespace string) (<-chan domain.WatchEvent, func())
}

type webhookPayload struct {
	Event       string    `json:"event"`
	Namespace   string    `json:"namespace"`
	Path        string    `json:"path"`
	Revision    int64     `json:"revision"`
	Timestamp   time.Time `json:"timestamp"`
	ContentHash string    `json:"content_hash,omitempty"`
}

type Dispatcher struct {
	repo      webhookLister
	publisher eventPublisher
	client    *http.Client

	historyMu sync.RWMutex
	history   map[string]*deliveryRingBuffer

	deliverySem chan struct{}

	stopOnce sync.Once
	stopCh   chan struct{}
	inflight sync.WaitGroup
}

func NewDispatcher(repo webhookLister, publisher eventPublisher) *Dispatcher {
	return &Dispatcher{ //nolint:exhaustruct // stopOnce/inflight have valid zero values
		repo:        repo,
		publisher:   publisher,
		client:      &http.Client{Timeout: httpRequestTimeout},
		history:     make(map[string]*deliveryRingBuffer),
		deliverySem: make(chan struct{}, maxConcurrentDeliveries),
		stopCh:      make(chan struct{}),
	}
}

// Run subscribes to watch events and fans them out to matching webhooks.
// Blocks until ctx (or the inner ctx set by Shutdown) is cancelled. Returns
// nil on cancellation; never returns a non-nil error — delivery failures are
// recorded in the per-webhook history ring buffer, not propagated.
//
// Implements lifecycle.Runner.
func (d *Dispatcher) Run(ctx context.Context) error {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("dispatcher: recovered from panic", "panic", r)
		}
	}()

	innerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Propagate Shutdown's stopCh to innerCtx so retry backoff sleeps and
	// in-flight HTTP requests unblock when Shutdown is called.
	stopWatcher := make(chan struct{})
	defer close(stopWatcher)

	go func() {
		select {
		case <-d.stopCh:
			cancel()
		case <-stopWatcher:
		}
	}()

	events, cleanup := d.publisher.Subscribe(innerCtx, "", "")
	defer cleanup()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return nil
			}

			d.inflight.Add(1)

			go func(ev domain.WatchEvent) {
				defer d.inflight.Done()
				d.dispatch(innerCtx, ev)
			}(event)
		case <-innerCtx.Done():
			return nil
		}
	}
}

// Shutdown signals Run to stop and waits for inflight deliveries to drain
// within the provided ctx budget. Cancelling the inner ctx unblocks retry
// backoff sleeps and aborts in-flight HTTP requests, so most deliveries
// terminate promptly and record a final attempt in the ring buffer.
//
// Idempotent: safe to call before Run, after Run has returned, or twice.
// Implements lifecycle.Resource.
func (d *Dispatcher) Shutdown(ctx context.Context) error {
	d.stopOnce.Do(func() { close(d.stopCh) })

	drained := make(chan struct{})

	go func() {
		d.inflight.Wait()
		close(drained)
	}()

	select {
	case <-drained:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("webhook dispatcher shutdown: %w", ctx.Err())
	}
}

func (d *Dispatcher) GetDeliveryHistory(webhookID string) []domain.DeliveryAttempt {
	d.historyMu.RLock()
	buf, ok := d.history[webhookID]
	d.historyMu.RUnlock()

	if !ok {
		return []domain.DeliveryAttempt{}
	}

	return buf.Snapshot()
}

func (d *Dispatcher) ClearHistory(webhookID string) {
	d.historyMu.Lock()
	defer d.historyMu.Unlock()

	delete(d.history, webhookID)
}

func (d *Dispatcher) dispatch(ctx context.Context, event domain.WatchEvent) {
	webhooks, err := d.repo.List(ctx)
	if err != nil {
		slog.Error("dispatcher: failed to list webhooks", "error", err)

		return
	}

	for _, wh := range webhooks {
		if wh.MatchesEvent(event) {
			select {
			case d.deliverySem <- struct{}{}:
				d.inflight.Add(1)

				go func(w *domain.Webhook) {
					defer d.inflight.Done()
					defer func() { <-d.deliverySem }()
					d.deliver(ctx, w, event)
				}(wh)
			case <-ctx.Done():
				return
			}
		}
	}
}

func (d *Dispatcher) deliver(ctx context.Context, wh *domain.Webhook, event domain.WatchEvent) {
	retryDelays := []time.Duration{0, 5 * time.Second, 30 * time.Second, 120 * time.Second}

	body, ok := d.buildPayloadBody(event)
	if !ok {
		return
	}

	for attempt := 1; attempt <= len(retryDelays); attempt++ {
		delay := retryDelays[attempt-1]

		if delay > 0 {
			jitter := cryptoJitter(delay)

			select {
			case <-time.After(jitter):
			case <-ctx.Done():
				return
			}
		}

		da := d.sendRequest(ctx, wh, body, attempt)
		d.getOrCreateBuffer(wh.ID).Push(da)

		if da.Success {
			return
		}
	}
}

func (d *Dispatcher) buildPayloadBody(event domain.WatchEvent) ([]byte, bool) {
	var eventStr string

	switch event.Type {
	case domain.EventTypeCreated:
		eventStr = "created"
	case domain.EventTypeUpdated:
		eventStr = "updated"
	case domain.EventTypeDeleted:
		eventStr = "deleted"
	default:
		return nil, false
	}

	payload := webhookPayload{
		Event:     eventStr,
		Namespace: event.Namespace,
		Path:      event.Path,
		Revision:  event.Revision,
		Timestamp: event.Timestamp,
	}

	if event.Config != nil {
		payload.ContentHash = event.Config.ContentHash
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, false
	}

	return body, true
}

func (d *Dispatcher) sendRequest(
	ctx context.Context,
	wh *domain.Webhook,
	body []byte,
	attempt int,
) domain.DeliveryAttempt {
	start := time.Now()

	reqCtx, cancel := context.WithTimeout(ctx, httpRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, wh.URL, bytes.NewReader(body))
	if err != nil {
		return domain.DeliveryAttempt{
			AttemptNumber: attempt,
			LatencyMS:     time.Since(start).Milliseconds(),
			Error:         fmt.Sprintf("create request: %s", err),
			Success:       false,
			Timestamp:     time.Now(),
		}
	}

	req.Header.Set("Content-Type", "application/json")

	if wh.Secret != "" {
		mac := hmac.New(sha256.New, []byte(wh.Secret))
		_, _ = mac.Write(body) // hash.Hash.Write never returns an error
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Elara-Signature", "sha256="+sig)
	}

	resp, err := d.client.Do(req)

	latency := time.Since(start).Milliseconds()

	if err != nil {
		return domain.DeliveryAttempt{
			AttemptNumber: attempt,
			LatencyMS:     latency,
			Error:         err.Error(),
			Success:       false,
			Timestamp:     time.Now(),
		}
	}

	defer func() { _ = resp.Body.Close() }()

	success := resp.StatusCode >= successStatusMin && resp.StatusCode < successStatusMax

	return domain.DeliveryAttempt{
		AttemptNumber: attempt,
		StatusCode:    resp.StatusCode,
		LatencyMS:     latency,
		Success:       success,
		Timestamp:     time.Now(),
	}
}

func (d *Dispatcher) getOrCreateBuffer(webhookID string) *deliveryRingBuffer {
	d.historyMu.RLock()
	buf, ok := d.history[webhookID]
	d.historyMu.RUnlock()

	if ok {
		return buf
	}

	d.historyMu.Lock()
	defer d.historyMu.Unlock()

	if buf, ok = d.history[webhookID]; ok {
		return buf
	}

	buf = newDeliveryRingBuffer(deliveryHistorySize)
	d.history[webhookID] = buf

	return buf
}

// Compile-time assertions.
var (
	_ lifecycle.Runner   = (*Dispatcher)(nil)
	_ lifecycle.Resource = (*Dispatcher)(nil)
)

// cryptoJitter returns delay ±20% using a cryptographically secure source.
func cryptoJitter(delay time.Duration) time.Duration {
	window := int64(delay) / jitterRange * jitterWindowFactor
	if window < 1 {
		return delay
	}

	n, err := rand.Int(rand.Reader, big.NewInt(window))
	if err != nil {
		return delay
	}

	return time.Duration(int64(delay)*4/jitterRange + n.Int64())
}
