package bbolt

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type WebhookRepo struct {
	manager *Manager
}

func NewWebhookRepo(manager *Manager) *WebhookRepo {
	return &WebhookRepo{manager: manager}
}

func (r *WebhookRepo) Create(ctx context.Context, w *domain.Webhook) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketWebhooks))
		// ...

		if w.ID == "" {
			w.ID = uuid.New().String()
		}

		key := []byte(w.ID)

		if b.Get(key) != nil {
			return domain.NewAlreadyExistsError("webhook", w.ID)
		}

		now := time.Now()
		w.CreatedAt = now
		w.UpdatedAt = now

		data, err := json.Marshal(domainToWebhookMeta(w)) //nolint:gosec // G117: intentionally storing secret in bbolt
		if err != nil {
			return fmt.Errorf("marshal webhook: %w", err)
		}

		return b.Put(key, data)
	})
	if err != nil {
		return fmt.Errorf("create webhook: %w", err)
	}

	return nil
}

func (r *WebhookRepo) Get(ctx context.Context, id string) (*domain.Webhook, error) {
	var w *domain.Webhook

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketWebhooks))
		data := b.Get([]byte(id))

		if data == nil {
			return domain.NewNotFoundError("webhook", id)
		}

		var m webhookMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("unmarshal webhook: %w", err)
		}

		w = webhookMetaToDomain(&m, id)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get webhook: %w", err)
	}

	return w, nil
}

func (r *WebhookRepo) Update(ctx context.Context, w *domain.Webhook) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketWebhooks))
		key := []byte(w.ID)

		data := b.Get(key)
		if data == nil {
			return domain.NewNotFoundError("webhook", w.ID)
		}

		var existing webhookMeta
		if err := json.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("unmarshal webhook: %w", err)
		}

		w.CreatedAt = existing.CreatedAt
		w.UpdatedAt = time.Now()

		if w.Secret == "" {
			w.Secret = existing.Secret
		}

		newData, err := json.Marshal( //nolint:gosec // G117: intentionally storing secret in bbolt
			domainToWebhookMeta(w),
		)
		if err != nil {
			return fmt.Errorf("marshal webhook: %w", err)
		}

		return b.Put(key, newData)
	})
	if err != nil {
		return fmt.Errorf("update webhook: %w", err)
	}

	return nil
}

func (r *WebhookRepo) Delete(ctx context.Context, id string) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketWebhooks))
		key := []byte(id)

		if b.Get(key) == nil {
			return domain.NewNotFoundError("webhook", id)
		}

		return b.Delete(key)
	})
	if err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}

	return nil
}

func (r *WebhookRepo) List(ctx context.Context) ([]*domain.Webhook, error) {
	var webhooks []*domain.Webhook

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketWebhooks))

		return b.ForEach(func(k, v []byte) error {
			var m webhookMeta
			if err := json.Unmarshal(v, &m); err != nil {
				return fmt.Errorf("unmarshal webhook %s: %w", k, err)
			}

			webhooks = append(webhooks, webhookMetaToDomain(&m, string(k)))

			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}

	return webhooks, nil
}
