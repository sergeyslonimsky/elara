// Package webhook is the bbolt-backed repository for domain.Webhook.
//
// The repository is intentionally "dumb": it owns no transaction lifetime,
// no defaults, no input preparation. Callers (services) mint IDs, set
// timestamps, normalize fields, and perform any read-modify-write needed
// to preserve fields across updates. The repo just persists what it is given.
//
// Repository errors are storage-level (storage.ErrResourceNotFound,
// storage.ErrResourceAlreadyExists). Callers translate them to domain
// errors at the usecase boundary.
package webhook

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/storage/internal"
	"github.com/sergeyslonimsky/elara/pkg/bbolt"
)

const bucketName = "webhooks"

type Repository struct {
	dbm bbolt.Manager
}

func NewRepository(dbm bbolt.Manager) *Repository {
	return &Repository{dbm: dbm}
}

func (r *Repository) Create(ctx context.Context, w *domain.Webhook) error {
	q := r.dbm.GetQuerier(ctx)

	if bbolt.Exists(q, bucketName, []byte(w.ID)) {
		return fmt.Errorf("webhook %s exists: %w", w.ID, storage.ErrResourceAlreadyExists)
	}

	if err := bbolt.Put(q, bucketName, []byte(w.ID), internal.DomainToMeta(w)); err != nil {
		return fmt.Errorf("put: %w", err)
	}

	return nil
}

func (r *Repository) Get(ctx context.Context, id string) (*domain.Webhook, error) {
	m, err := bbolt.Get[internal.WebhookMeta](r.dbm.GetQuerier(ctx), bucketName, []byte(id))
	if errors.Is(err, bbolt.ErrNotFound) {
		return nil, fmt.Errorf("webhook %s: %w", id, storage.ErrResourceNotFound)
	}

	if err != nil {
		return nil, fmt.Errorf("get: %w", err)
	}

	return internal.MetaToDomain(m, id), nil
}

func (r *Repository) Update(ctx context.Context, w *domain.Webhook) error {
	q := r.dbm.GetQuerier(ctx)

	if !bbolt.Exists(q, bucketName, []byte(w.ID)) {
		return fmt.Errorf("webhook %s: %w", w.ID, storage.ErrResourceNotFound)
	}

	if err := bbolt.Put(q, bucketName, []byte(w.ID), internal.DomainToMeta(w)); err != nil {
		return fmt.Errorf("put: %w", err)
	}

	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	q := r.dbm.GetQuerier(ctx)

	if !bbolt.Exists(q, bucketName, []byte(id)) {
		return fmt.Errorf("webhook %s: %w", id, storage.ErrResourceNotFound)
	}

	if err := bbolt.Delete(q, bucketName, []byte(id)); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	return nil
}

func (r *Repository) List(ctx context.Context) ([]*domain.Webhook, error) {
	var out []*domain.Webhook

	codec := bbolt.JSONCodec[internal.WebhookMeta]{}

	err := r.dbm.GetQuerier(ctx).Bucket(bucketName).ForEach(func(k, v []byte) error {
		var m internal.WebhookMeta
		if err := codec.Unmarshal(v, &m); err != nil {
			return fmt.Errorf("decode %s: %w", k, err)
		}

		out = append(out, internal.MetaToDomain(m, string(k)))

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	return out, nil
}
