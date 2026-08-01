package internal_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
	storageinternal "github.com/sergeyslonimsky/elara/internal/storage/internal"
)

func TestDomainToMeta(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name    string
		webhook *domain.Webhook
		want    storageinternal.WebhookMeta
	}{
		{
			name: "full webhook",
			webhook: &domain.Webhook{
				ID:              "wh-1",
				URL:             "https://example.com/hook",
				NamespaceFilter: "default",
				PathPrefix:      "/foo",
				Events:          []domain.WebhookEventType{domain.WebhookEventCreated, domain.WebhookEventDeleted},
				Secret:          "shh",
				Enabled:         true,
				CreatedAt:       now,
				UpdatedAt:       now,
			},
			want: storageinternal.WebhookMeta{
				URL:             "https://example.com/hook",
				NamespaceFilter: "default",
				PathPrefix:      "/foo",
				Events:          []string{"created", "deleted"},
				Secret:          "shh",
				Enabled:         true,
				CreatedAt:       now,
				UpdatedAt:       now,
			},
		},
		{
			name:    "nil events becomes empty slice",
			webhook: &domain.Webhook{},
			want: storageinternal.WebhookMeta{
				Events: []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.DomainToMeta(tt.webhook)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMetaToDomain(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name string
		meta storageinternal.WebhookMeta
		id   string
		want *domain.Webhook
	}{
		{
			name: "full meta",
			meta: storageinternal.WebhookMeta{
				URL:             "https://example.com/hook",
				NamespaceFilter: "default",
				PathPrefix:      "/foo",
				Events:          []string{"created", "updated"},
				Secret:          "shh",
				Enabled:         true,
				CreatedAt:       now,
				UpdatedAt:       now,
			},
			id: "wh-1",
			want: &domain.Webhook{
				ID:              "wh-1",
				URL:             "https://example.com/hook",
				NamespaceFilter: "default",
				PathPrefix:      "/foo",
				Events:          []domain.WebhookEventType{domain.WebhookEventCreated, domain.WebhookEventUpdated},
				Secret:          "shh",
				Enabled:         true,
				CreatedAt:       now,
				UpdatedAt:       now,
			},
		},
		{
			name: "nil events becomes empty slice",
			meta: storageinternal.WebhookMeta{},
			id:   "wh-2",
			want: &domain.Webhook{
				ID:     "wh-2",
				Events: []domain.WebhookEventType{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.MetaToDomain(tt.meta, tt.id)
			assert.Equal(t, tt.want, got)
		})
	}
}
