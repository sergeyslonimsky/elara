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

	mu      sync.RWMutex
	history map[string]*deliveryRingBuffer

	deliverySem chan struct{}
	stopOnce    sync.Once
	stopCh      chan struct{}
}

func NewDispatcher(repo webhookLister, publisher eventPublisher) *Dispatcher {
	return &Dispatcher{
		repo:        repo,
		publisher:   publisher,
		client:      &http.Client{Timeout: httpRequestTimeout},
		history:     make(map[string]*deliveryRingBuffer),
		deliverySem: make(chan struct{}, maxConcurrentDeliveries),
		stopCh:      make(chan struct{}),
	}
}

func (d *Dispatcher) Start(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("dispatcher: recovered from panic", "panic", r)
		}
	}()

	events, cleanup := d.publisher.Subscribe(ctx, "", "")
	defer cleanup()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}

			go d.dispatch(ctx, event)
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		}
	}
}

func (d *Dispatcher) Stop() {
	d.stopOnce.Do(func() { close(d.stopCh) })
}

func (d *Dispatcher) GetDeliveryHistory(webhookID string) []domain.DeliveryAttempt {
	d.mu.RLock()
	buf, ok := d.history[webhookID]
	d.mu.RUnlock()

	if !ok {
		return []domain.DeliveryAttempt{}
	}

	return buf.Snapshot()
}

func (d *Dispatcher) ClearHistory(webhookID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

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
				go func(w *domain.Webhook) {
					defer func() { <-d.deliverySem }()
					d.deliver(ctx, w, event)
				}(wh)
			case <-ctx.Done():
				return
			case <-d.stopCh:
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
			case <-d.stopCh:
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
	d.mu.RLock()
	buf, ok := d.history[webhookID]
	d.mu.RUnlock()

	if ok {
		return buf
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if buf, ok = d.history[webhookID]; ok {
		return buf
	}

	buf = newDeliveryRingBuffer(deliveryHistorySize)
	d.history[webhookID] = buf

	return buf
}

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
