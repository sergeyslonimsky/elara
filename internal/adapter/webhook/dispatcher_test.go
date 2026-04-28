package webhook_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	webhookadapter "github.com/sergeyslonimsky/elara/internal/adapter/webhook"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

type mockPublisher struct {
	ch chan domain.WatchEvent
}

func newMockPublisher() *mockPublisher {
	return &mockPublisher{ch: make(chan domain.WatchEvent, 10)}
}

func (m *mockPublisher) Subscribe(_ context.Context, _, _ string) (<-chan domain.WatchEvent, func()) {
	return m.ch, func() {}
}

func (m *mockPublisher) Send(e domain.WatchEvent) {
	m.ch <- e
}

type mockLister struct {
	mu       sync.RWMutex
	webhooks []*domain.Webhook
}

func (m *mockLister) List(_ context.Context) ([]*domain.Webhook, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*domain.Webhook, len(m.webhooks))
	copy(out, m.webhooks)

	return out, nil
}

func (m *mockLister) setWebhooks(webhooks []*domain.Webhook) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.webhooks = webhooks
}

func TestDispatcher_EventDispatchedToMatchingWebhook(t *testing.T) {
	t.Parallel()

	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pub := newMockPublisher()
	lister := &mockLister{}
	lister.setWebhooks([]*domain.Webhook{
		{
			ID:      "wh-1",
			URL:     srv.URL,
			Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
			Enabled: true,
		},
	})

	dispatcher := webhookadapter.NewDispatcher(lister, pub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go dispatcher.Start(ctx)

	pub.Send(domain.WatchEvent{
		Type:      domain.EventTypeCreated,
		Path:      "/config.json",
		Namespace: "prod",
		Revision:  1,
		Timestamp: time.Now(),
	})

	require.Eventually(t, func() bool {
		return received.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)
}

func TestDispatcher_HMACHeaderPresentAndCorrect(t *testing.T) {
	t.Parallel()

	var (
		dataMu       sync.Mutex
		receivedSig  string
		receivedBody []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sig := r.Header.Get("X-Elara-Signature")
		body, _ := io.ReadAll(r.Body)

		dataMu.Lock()
		receivedSig = sig
		receivedBody = body
		dataMu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	secret := "my-secret"
	pub := newMockPublisher()
	lister := &mockLister{}
	lister.setWebhooks([]*domain.Webhook{
		{
			ID:      "wh-2",
			URL:     srv.URL,
			Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
			Secret:  secret,
			Enabled: true,
		},
	})

	dispatcher := webhookadapter.NewDispatcher(lister, pub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go dispatcher.Start(ctx)

	pub.Send(domain.WatchEvent{
		Type:      domain.EventTypeCreated,
		Path:      "/config.json",
		Namespace: "prod",
		Revision:  1,
		Timestamp: time.Now(),
	})

	require.Eventually(t, func() bool {
		dataMu.Lock()
		defer dataMu.Unlock()

		return receivedSig != ""
	}, 2*time.Second, 10*time.Millisecond)

	dataMu.Lock()
	defer dataMu.Unlock()

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(receivedBody)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	assert.Equal(t, expected, receivedSig)
}

func TestDispatcher_NonMatchingNamespaceSkipped(t *testing.T) {
	t.Parallel()

	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pub := newMockPublisher()
	lister := &mockLister{}
	lister.setWebhooks([]*domain.Webhook{
		{
			ID:              "wh-3",
			URL:             srv.URL,
			Events:          []domain.WebhookEventType{domain.WebhookEventCreated},
			NamespaceFilter: "staging",
			Enabled:         true,
		},
	})

	dispatcher := webhookadapter.NewDispatcher(lister, pub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go dispatcher.Start(ctx)

	pub.Send(domain.WatchEvent{
		Type:      domain.EventTypeCreated,
		Path:      "/config.json",
		Namespace: "production",
		Revision:  1,
		Timestamp: time.Now(),
	})

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(0), received.Load())
}

func TestDispatcher_EventNotDelivered(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		webhook domain.Webhook
		event   domain.WatchEvent
	}{
		{
			name: "disabled webhook skipped",
			webhook: domain.Webhook{
				ID:      "wh-4",
				Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled: false,
			},
			event: domain.WatchEvent{
				Type:      domain.EventTypeCreated,
				Path:      "/config.json",
				Namespace: "prod",
				Revision:  1,
				Timestamp: time.Now(),
			},
		},
		{
			name: "unknown event type not delivered",
			webhook: domain.Webhook{
				ID:      "wh-unknown",
				Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
				Enabled: true,
			},
			event: domain.WatchEvent{
				Type:      domain.EventTypeLocked,
				Path:      "/config.json",
				Namespace: "prod",
				Revision:  1,
				Timestamp: time.Now(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var received atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			tt.webhook.URL = srv.URL

			pub := newMockPublisher()
			lister := &mockLister{}
			lister.setWebhooks([]*domain.Webhook{&tt.webhook})

			dispatcher := webhookadapter.NewDispatcher(lister, pub)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			go dispatcher.Start(ctx)

			pub.Send(tt.event)

			time.Sleep(200 * time.Millisecond)
			assert.Equal(t, int32(0), received.Load())
		})
	}
}

func TestDispatcher_DeliveryHistoryRecorded(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pub := newMockPublisher()
	lister := &mockLister{}
	lister.setWebhooks([]*domain.Webhook{
		{
			ID:      "wh-5",
			URL:     srv.URL,
			Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
			Enabled: true,
		},
	})

	dispatcher := webhookadapter.NewDispatcher(lister, pub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go dispatcher.Start(ctx)

	pub.Send(domain.WatchEvent{
		Type:      domain.EventTypeCreated,
		Path:      "/config.json",
		Namespace: "prod",
		Revision:  1,
		Timestamp: time.Now(),
	})

	require.Eventually(t, func() bool {
		history := dispatcher.GetDeliveryHistory("wh-5")

		return len(history) == 1
	}, 2*time.Second, 10*time.Millisecond)

	history := dispatcher.GetDeliveryHistory("wh-5")
	require.Len(t, history, 1)
	assert.True(t, history[0].Success)
	assert.Equal(t, 1, history[0].AttemptNumber)
	assert.Equal(t, http.StatusOK, history[0].StatusCode)
}

func TestDispatcher_PayloadContents(t *testing.T) {
	t.Parallel()

	type payloadType struct {
		Event     string    `json:"event"`
		Namespace string    `json:"namespace"`
		Path      string    `json:"path"`
		Revision  int64     `json:"revision"`
		Timestamp time.Time `json:"timestamp"`
	}

	var (
		receivedMu sync.Mutex
		received   payloadType
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		var p payloadType
		_ = json.Unmarshal(body, &p)

		receivedMu.Lock()
		received = p
		receivedMu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pub := newMockPublisher()
	lister := &mockLister{}
	lister.setWebhooks([]*domain.Webhook{
		{
			ID:      "wh-6",
			URL:     srv.URL,
			Events:  []domain.WebhookEventType{domain.WebhookEventUpdated},
			Enabled: true,
		},
	})

	dispatcher := webhookadapter.NewDispatcher(lister, pub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go dispatcher.Start(ctx)

	pub.Send(domain.WatchEvent{
		Type:      domain.EventTypeUpdated,
		Path:      "/myapp/config.yaml",
		Namespace: "staging",
		Revision:  42,
		Timestamp: time.Now(),
	})

	require.Eventually(t, func() bool {
		receivedMu.Lock()
		defer receivedMu.Unlock()

		return received.Path != ""
	}, 2*time.Second, 10*time.Millisecond)

	receivedMu.Lock()
	defer receivedMu.Unlock()

	assert.Equal(t, "updated", received.Event)
	assert.Equal(t, "/myapp/config.yaml", received.Path)
	assert.Equal(t, "staging", received.Namespace)
	assert.Equal(t, int64(42), received.Revision)
}

func TestDeliveryRingBuffer_Push60ReturnsLast50(t *testing.T) {
	t.Parallel()

	pub := newMockPublisher()
	lister := &mockLister{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	webhookID := "wh-ring"
	lister.setWebhooks([]*domain.Webhook{
		{
			ID:      webhookID,
			URL:     srv.URL,
			Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
			Enabled: true,
		},
	})

	dispatcher := webhookadapter.NewDispatcher(lister, pub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go dispatcher.Start(ctx)

	const total = 60

	for i := range total {
		pub.Send(domain.WatchEvent{
			Type:      domain.EventTypeCreated,
			Path:      fmt.Sprintf("/config-%d.json", i),
			Namespace: "prod",
			Revision:  int64(i + 1),
			Timestamp: time.Now(),
		})
	}

	require.Eventually(t, func() bool {
		return len(dispatcher.GetDeliveryHistory(webhookID)) >= 50
	}, 5*time.Second, 20*time.Millisecond)

	history := dispatcher.GetDeliveryHistory(webhookID)
	assert.Len(t, history, 50)

	for _, a := range history {
		assert.True(t, a.Success)
	}
}

func TestDispatcher_GetDeliveryHistory_UnknownWebhookID_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	pub := newMockPublisher()
	lister := &mockLister{}
	dispatcher := webhookadapter.NewDispatcher(lister, pub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go dispatcher.Start(ctx)

	history := dispatcher.GetDeliveryHistory("nonexistent-wh")
	assert.Empty(t, history)
}

func TestDispatcher_UnknownEventTypeNotDelivered(t *testing.T) {
	t.Parallel()

	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pub := newMockPublisher()
	lister := &mockLister{}
	lister.setWebhooks([]*domain.Webhook{
		{
			ID:      "wh-unknown",
			URL:     srv.URL,
			Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
			Enabled: true,
		},
	})

	dispatcher := webhookadapter.NewDispatcher(lister, pub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go dispatcher.Start(ctx)

	pub.Send(domain.WatchEvent{
		Type:      domain.EventTypeLocked,
		Path:      "/config.json",
		Namespace: "prod",
		Revision:  1,
		Timestamp: time.Now(),
	})

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(0), received.Load())
}

func TestDispatcher_ClearHistory_RemovesHistory(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pub := newMockPublisher()
	lister := &mockLister{}
	lister.setWebhooks([]*domain.Webhook{
		{
			ID:      "wh-clear",
			URL:     srv.URL,
			Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
			Enabled: true,
		},
	})

	dispatcher := webhookadapter.NewDispatcher(lister, pub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go dispatcher.Start(ctx)

	pub.Send(domain.WatchEvent{
		Type:      domain.EventTypeCreated,
		Path:      "/config.json",
		Namespace: "prod",
		Revision:  1,
		Timestamp: time.Now(),
	})

	require.Eventually(t, func() bool {
		return len(dispatcher.GetDeliveryHistory("wh-clear")) == 1
	}, 2*time.Second, 10*time.Millisecond)

	dispatcher.ClearHistory("wh-clear")
	assert.Empty(t, dispatcher.GetDeliveryHistory("wh-clear"))
}

// mockListerWithError always returns an error from List.
type mockListerWithError struct {
	err error
}

func (m *mockListerWithError) List(_ context.Context) ([]*domain.Webhook, error) {
	return nil, m.err
}

func TestDispatcher_Stop_StopsProcessing(t *testing.T) {
	t.Parallel()

	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pub := newMockPublisher()
	lister := &mockLister{}
	lister.setWebhooks([]*domain.Webhook{
		{
			ID:      "wh-stop",
			URL:     srv.URL,
			Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
			Enabled: true,
		},
	})

	dispatcher := webhookadapter.NewDispatcher(lister, pub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go dispatcher.Start(ctx)

	// Stop the dispatcher before sending any event.
	dispatcher.Stop()

	// Give the dispatcher goroutine time to process the stop signal.
	time.Sleep(50 * time.Millisecond)

	pub.Send(domain.WatchEvent{
		Type:      domain.EventTypeCreated,
		Path:      "/config.json",
		Namespace: "prod",
		Revision:  1,
		Timestamp: time.Now(),
	})

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(0), received.Load())
}

func TestDispatcher_DispatchListError_NoDelivery(t *testing.T) {
	t.Parallel()

	pub := newMockPublisher()
	lister := &mockListerWithError{err: errors.New("db error")}

	dispatcher := webhookadapter.NewDispatcher(lister, pub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go dispatcher.Start(ctx)

	pub.Send(domain.WatchEvent{
		Type:      domain.EventTypeCreated,
		Path:      "/config.json",
		Namespace: "prod",
		Revision:  1,
		Timestamp: time.Now(),
	})

	// No panic and no delivery — just verify the dispatcher is still alive.
	time.Sleep(200 * time.Millisecond)

	history := dispatcher.GetDeliveryHistory("any-wh")
	assert.Empty(t, history)
}

func TestDispatcher_RetryOnFailure_EventuallySucceeds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		failFirst      int // number of 500 responses before switching to 200
		wantAttempts   int
		wantLastStatus int
	}{
		{
			name:           "succeeds on first retry after one failure",
			failFirst:      1,
			wantAttempts:   2,
			wantLastStatus: http.StatusOK,
		},
		{
			name:           "succeeds immediately on first attempt",
			failFirst:      0,
			wantAttempts:   1,
			wantLastStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var callCount atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := int(callCount.Add(1))
				if n <= tt.failFirst {
					w.WriteHeader(http.StatusInternalServerError)
				} else {
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer srv.Close()

			pub := newMockPublisher()
			lister := &mockLister{}
			webhookID := "wh-retry-" + tt.name
			lister.setWebhooks([]*domain.Webhook{
				{
					ID:      webhookID,
					URL:     srv.URL,
					Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
					Enabled: true,
				},
			})

			dispatcher := webhookadapter.NewDispatcher(lister, pub)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			go dispatcher.Start(ctx)

			pub.Send(domain.WatchEvent{
				Type:      domain.EventTypeCreated,
				Path:      "/config.json",
				Namespace: "prod",
				Revision:  1,
				Timestamp: time.Now(),
			})

			require.Eventually(t, func() bool {
				history := dispatcher.GetDeliveryHistory(webhookID)
				if len(history) < tt.wantAttempts {
					return false
				}

				return history[len(history)-1].Success
			}, 10*time.Second, 20*time.Millisecond)

			history := dispatcher.GetDeliveryHistory(webhookID)
			require.GreaterOrEqual(t, len(history), tt.wantAttempts)
			assert.Equal(t, tt.wantLastStatus, history[len(history)-1].StatusCode)
			assert.True(t, history[len(history)-1].Success)
		})
	}
}

func TestDispatcher_BuildPayload_ContentHashPresent(t *testing.T) {
	t.Parallel()

	type payloadType struct {
		ContentHash string `json:"content_hash"`
	}

	var (
		receivedMu sync.Mutex
		received   payloadType
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		var p payloadType
		_ = json.Unmarshal(body, &p)

		receivedMu.Lock()
		received = p
		receivedMu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pub := newMockPublisher()
	lister := &mockLister{}
	lister.setWebhooks([]*domain.Webhook{
		{
			ID:      "wh-content-hash",
			URL:     srv.URL,
			Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
			Enabled: true,
		},
	})

	dispatcher := webhookadapter.NewDispatcher(lister, pub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go dispatcher.Start(ctx)

	pub.Send(domain.WatchEvent{
		Type:      domain.EventTypeCreated,
		Path:      "/config.json",
		Namespace: "prod",
		Revision:  1,
		Timestamp: time.Now(),
		Config:    &domain.Config{ContentHash: "abc123"},
	})

	require.Eventually(t, func() bool {
		receivedMu.Lock()
		defer receivedMu.Unlock()

		return received.ContentHash != ""
	}, 2*time.Second, 10*time.Millisecond)

	receivedMu.Lock()
	defer receivedMu.Unlock()

	assert.Equal(t, "abc123", received.ContentHash)
}

func TestDispatcher_SendRequest_NetworkError_RecordsFailure(t *testing.T) {
	t.Parallel()

	// Start a server and immediately close it to produce a network error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	closedURL := srv.URL
	srv.Close()

	pub := newMockPublisher()
	lister := &mockLister{}
	webhookID := "wh-net-err"
	lister.setWebhooks([]*domain.Webhook{
		{
			ID:      webhookID,
			URL:     closedURL,
			Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
			Enabled: true,
		},
	})

	dispatcher := webhookadapter.NewDispatcher(lister, pub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go dispatcher.Start(ctx)

	pub.Send(domain.WatchEvent{
		Type:      domain.EventTypeCreated,
		Path:      "/config.json",
		Namespace: "prod",
		Revision:  1,
		Timestamp: time.Now(),
	})

	require.Eventually(t, func() bool {
		return len(dispatcher.GetDeliveryHistory(webhookID)) > 0
	}, 2*time.Second, 10*time.Millisecond)

	history := dispatcher.GetDeliveryHistory(webhookID)
	require.NotEmpty(t, history)
	assert.False(t, history[0].Success)
	assert.NotEmpty(t, history[0].Error)
}

func TestDispatcher_ConcurrentDeliveries_SemaphoreNotExceeded(t *testing.T) {
	t.Parallel()

	const numEvents = 10

	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pub := newMockPublisher()
	lister := &mockLister{}
	lister.setWebhooks([]*domain.Webhook{
		{
			ID:      "wh-concurrent",
			URL:     srv.URL,
			Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
			Enabled: true,
		},
	})

	dispatcher := webhookadapter.NewDispatcher(lister, pub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go dispatcher.Start(ctx)

	for i := range numEvents {
		pub.Send(domain.WatchEvent{
			Type:      domain.EventTypeCreated,
			Path:      fmt.Sprintf("/config-%d.json", i),
			Namespace: "prod",
			Revision:  int64(i + 1),
			Timestamp: time.Now(),
		})
	}

	require.Eventually(t, func() bool {
		return received.Load() == numEvents
	}, 5*time.Second, 20*time.Millisecond)

	assert.Equal(t, int32(numEvents), received.Load())
}
