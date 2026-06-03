package internal

import (
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type WebhookMeta struct {
	URL             string    `json:"url"`
	NamespaceFilter string    `json:"namespace_filter"`
	PathPrefix      string    `json:"path_prefix"`
	Events          []string  `json:"events"`
	Secret          string    `json:"secret"`
	Enabled         bool      `json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func DomainToMeta(w *domain.Webhook) WebhookMeta {
	events := make([]string, len(w.Events))
	for i, e := range w.Events {
		events[i] = string(e)
	}

	return WebhookMeta{
		URL:             w.URL,
		NamespaceFilter: w.NamespaceFilter,
		PathPrefix:      w.PathPrefix,
		Events:          events,
		Secret:          w.Secret,
		Enabled:         w.Enabled,
		CreatedAt:       w.CreatedAt,
		UpdatedAt:       w.UpdatedAt,
	}
}

func MetaToDomain(m WebhookMeta, id string) *domain.Webhook {
	events := make([]domain.WebhookEventType, len(m.Events))
	for i, e := range m.Events {
		events[i] = domain.WebhookEventType(e)
	}

	return &domain.Webhook{
		ID:              id,
		URL:             m.URL,
		NamespaceFilter: m.NamespaceFilter,
		PathPrefix:      m.PathPrefix,
		Events:          events,
		Secret:          m.Secret,
		Enabled:         m.Enabled,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}
