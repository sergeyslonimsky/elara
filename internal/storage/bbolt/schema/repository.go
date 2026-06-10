// Package schema is the bbolt-backed repository for
// domain.SchemaAttachment.
package schema

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/storage/internal"
	"github.com/sergeyslonimsky/elara/pkg/bbolt"
)

const bucketName = "schemas"

type Repository struct {
	dbm bbolt.Manager
}

func NewRepository(dbm bbolt.Manager) *Repository {
	return &Repository{dbm: dbm}
}

// Attach upserts a schema attachment. On update CreatedAt is preserved from
// the existing record; on insert it defaults to time.Now() when unset.
func (r *Repository) Attach(ctx context.Context, s *domain.SchemaAttachment) error {
	err := r.dbm.WithTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)
		key := schemaKey(s.Namespace, s.PathPattern)

		existing, err := bbolt.Get[internal.SchemaMeta](q, bucketName, key)
		switch {
		case err == nil:
			s.CreatedAt = existing.CreatedAt
		case errors.Is(err, bbolt.ErrNotFound):
			if s.CreatedAt.IsZero() {
				s.CreatedAt = time.Now()
			}
		default:
			return fmt.Errorf("get: %w", err)
		}

		if err := bbolt.Put(q, bucketName, key, internal.DomainToSchemaMeta(s)); err != nil {
			return fmt.Errorf("put: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("attach schema: %w", err)
	}

	return nil
}

func (r *Repository) Detach(ctx context.Context, namespace, pathPattern string) error {
	q := r.dbm.GetQuerier(ctx)
	key := schemaKey(namespace, pathPattern)

	if !bbolt.Exists(q, bucketName, key) {
		return fmt.Errorf("schema %s: %w", pathPattern, storage.ErrResourceNotFound)
	}

	if err := bbolt.Delete(q, bucketName, key); err != nil {
		return fmt.Errorf("detach schema: %w", err)
	}

	return nil
}

func (r *Repository) Get(
	ctx context.Context,
	namespace, pathPattern string,
) (*domain.SchemaAttachment, error) {
	m, err := bbolt.Get[internal.SchemaMeta](
		r.dbm.GetQuerier(ctx),
		bucketName,
		schemaKey(namespace, pathPattern),
	)
	if errors.Is(err, bbolt.ErrNotFound) {
		return nil, fmt.Errorf("schema %s: %w", pathPattern, storage.ErrResourceNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get schema: %w", err)
	}

	return internal.SchemaMetaToDomain(m, namespace, pathPattern), nil
}

func (r *Repository) List(ctx context.Context, namespace string) ([]*domain.SchemaAttachment, error) {
	var attachments []*domain.SchemaAttachment

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		bucket := r.dbm.GetQuerier(ctx).Bucket(bucketName)
		codec := bbolt.JSONCodec[internal.SchemaMeta]{}
		prefix := schemaKeyPrefix(namespace)
		c := bucket.Cursor()

		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			_, pathPattern, _ := bytes.Cut(k, []byte{keySep})

			var m internal.SchemaMeta
			if err := codec.Unmarshal(v, &m); err != nil {
				return fmt.Errorf("decode %s: %w", k, err)
			}

			attachments = append(
				attachments,
				internal.SchemaMetaToDomain(m, namespace, string(pathPattern)),
			)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}

	return attachments, nil
}
