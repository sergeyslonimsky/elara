package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestWebhook_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		webhook domain.Webhook
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid webhook",
			webhook: domain.Webhook{
				URL:    "https://example.com/hook",
				Events: []domain.WebhookEventType{domain.WebhookEventCreated},
			},
			wantErr: false,
		},
		{
			name: "valid webhook with http",
			webhook: domain.Webhook{
				URL:    "http://example.com/hook",
				Events: []domain.WebhookEventType{domain.WebhookEventCreated, domain.WebhookEventUpdated},
			},
			wantErr: false,
		},
		{
			name: "missing URL",
			webhook: domain.Webhook{
				Events: []domain.WebhookEventType{domain.WebhookEventCreated},
			},
			wantErr: true,
			errMsg:  "url",
		},
		{
			name: "invalid URL scheme ftp",
			webhook: domain.Webhook{
				URL:    "ftp://example.com/hook",
				Events: []domain.WebhookEventType{domain.WebhookEventCreated},
			},
			wantErr: true,
			errMsg:  "url",
		},
		{
			name: "no events",
			webhook: domain.Webhook{
				URL:    "https://example.com/hook",
				Events: []domain.WebhookEventType{},
			},
			wantErr: true,
			errMsg:  "events",
		},
		{
			name: "nil events",
			webhook: domain.Webhook{
				URL: "https://example.com/hook",
			},
			wantErr: true,
			errMsg:  "events",
		},
		{
			name: "unknown event type",
			webhook: domain.Webhook{
				URL:    "https://example.com/hook",
				Events: []domain.WebhookEventType{"locked"},
			},
			wantErr: true,
			errMsg:  "events",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.webhook.Validate()

			if tt.wantErr {
				require.Error(t, err)
				assert.True(t, domain.IsValidationError(err))

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWebhook_MatchesEvent(t *testing.T) {
	t.Parallel()

	baseWebhook := domain.Webhook{
		ID:      "wh-1",
		URL:     "https://example.com/hook",
		Events:  []domain.WebhookEventType{domain.WebhookEventCreated, domain.WebhookEventUpdated},
		Enabled: true,
	}

	baseEvent := domain.WatchEvent{
		Type:      domain.EventTypeCreated,
		Path:      "/services/api/config.json",
		Namespace: "production",
		Revision:  1,
		Timestamp: time.Now(),
	}

	tests := []struct {
		name    string
		webhook domain.Webhook
		event   domain.WatchEvent
		want    bool
	}{
		{
			name: "disabled webhook returns false",
			webhook: func() domain.Webhook {
				w := baseWebhook
				w.Enabled = false

				return w
			}(),
			event: baseEvent,
			want:  false,
		},
		{
			name:    "wrong event type filtered",
			webhook: baseWebhook,
			event: func() domain.WatchEvent {
				e := baseEvent
				e.Type = domain.EventTypeDeleted

				return e
			}(),
			want: false,
		},
		{
			name:    "lock event ignored",
			webhook: baseWebhook,
			event: func() domain.WatchEvent {
				e := baseEvent
				e.Type = domain.EventTypeLocked

				return e
			}(),
			want: false,
		},
		{
			name:    "namespace unlock event ignored",
			webhook: baseWebhook,
			event: func() domain.WatchEvent {
				e := baseEvent
				e.Type = domain.EventTypeNamespaceUnlocked

				return e
			}(),
			want: false,
		},
		{
			name: "namespace filter match",
			webhook: func() domain.Webhook {
				w := baseWebhook
				w.NamespaceFilter = "production"

				return w
			}(),
			event: baseEvent,
			want:  true,
		},
		{
			name: "namespace filter mismatch",
			webhook: func() domain.Webhook {
				w := baseWebhook
				w.NamespaceFilter = "staging"

				return w
			}(),
			event: baseEvent,
			want:  false,
		},
		{
			name: "path prefix filter match",
			webhook: func() domain.Webhook {
				w := baseWebhook
				w.PathPrefix = "/services"

				return w
			}(),
			event: baseEvent,
			want:  true,
		},
		{
			name: "path prefix filter mismatch",
			webhook: func() domain.Webhook {
				w := baseWebhook
				w.PathPrefix = "/infra"

				return w
			}(),
			event: baseEvent,
			want:  false,
		},
		{
			name:    "all match case",
			webhook: baseWebhook,
			event:   baseEvent,
			want:    true,
		},
		{
			name:    "updated event matches",
			webhook: baseWebhook,
			event: func() domain.WatchEvent {
				e := baseEvent
				e.Type = domain.EventTypeUpdated

				return e
			}(),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.webhook.MatchesEvent(tt.event)
			assert.Equal(t, tt.want, got)
		})
	}
}
