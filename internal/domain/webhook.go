package domain

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
)

type WebhookEventType string

const (
	WebhookEventCreated WebhookEventType = "created"
	WebhookEventUpdated WebhookEventType = "updated"
	WebhookEventDeleted WebhookEventType = "deleted"
)

type Webhook struct {
	ID              string
	URL             string
	NamespaceFilter string
	PathPrefix      string
	Events          []WebhookEventType
	Secret          string
	Enabled         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type DeliveryAttempt struct {
	AttemptNumber int
	StatusCode    int
	LatencyMS     int64
	Error         string
	Success       bool
	Timestamp     time.Time
}

func (w *Webhook) Validate() error {
	if w.URL == "" {
		return NewValidationError("url", "url is required")
	}

	u, err := url.Parse(w.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return NewValidationError("url", fmt.Sprintf("url must be a valid http or https URL, got %q", w.URL))
	}

	if len(w.Events) == 0 {
		return NewValidationError("events", "at least one event is required")
	}

	for _, e := range w.Events {
		if e != WebhookEventCreated && e != WebhookEventUpdated && e != WebhookEventDeleted {
			return NewValidationError("events", fmt.Sprintf("unknown event type %q", e))
		}
	}

	return nil
}

func webhookEventFromWatchEvent(event WatchEvent) (WebhookEventType, bool) {
	switch event.Type {
	case EventTypeCreated:
		return WebhookEventCreated, true
	case EventTypeUpdated:
		return WebhookEventUpdated, true
	case EventTypeDeleted:
		return WebhookEventDeleted, true
	default:
		return "", false
	}
}

func (w *Webhook) MatchesEvent(event WatchEvent) bool {
	if !w.Enabled {
		return false
	}

	webhookType, ok := webhookEventFromWatchEvent(event)
	if !ok {
		return false
	}

	if !slices.Contains(w.Events, webhookType) {
		return false
	}

	if w.NamespaceFilter != "" && event.Namespace != w.NamespaceFilter {
		return false
	}

	if w.PathPrefix != "" && !strings.HasPrefix(event.Path, w.PathPrefix) {
		return false
	}

	return true
}
