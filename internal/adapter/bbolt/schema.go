package bbolt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type SchemaRepo struct {
	store *Store
}

func NewSchemaRepo(store *Store) *SchemaRepo {
	return &SchemaRepo{store: store}
}

func (r *SchemaRepo) Attach(_ context.Context, s *domain.SchemaAttachment) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketSchemas))
		key := schemaKey(s.Namespace, s.PathPattern)

		// Upsert: preserve original CreatedAt when updating an existing attachment.
		if existing := b.Get(key); existing != nil {
			var m schemaMeta
			if err := json.Unmarshal(existing, &m); err == nil {
				s.CreatedAt = m.CreatedAt
			}
		} else if s.CreatedAt.IsZero() {
			s.CreatedAt = time.Now()
		}

		data, err := json.Marshal(domainToSchemaMeta(s))
		if err != nil {
			return fmt.Errorf("marshal schema: %w", err)
		}

		return b.Put(key, data)
	})
	if err != nil {
		return fmt.Errorf("attach schema: %w", err)
	}

	return nil
}

func (r *SchemaRepo) Detach(_ context.Context, namespace, pathPattern string) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketSchemas))
		key := schemaKey(namespace, pathPattern)

		if b.Get(key) == nil {
			return domain.NewNotFoundError("schema", pathPattern)
		}

		return b.Delete(key)
	})
	if err != nil {
		return fmt.Errorf("detach schema: %w", err)
	}

	return nil
}

func (r *SchemaRepo) Get(_ context.Context, namespace, pathPattern string) (*domain.SchemaAttachment, error) {
	var attachment *domain.SchemaAttachment

	err := r.store.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketSchemas))
		key := schemaKey(namespace, pathPattern)
		data := b.Get(key)

		if data == nil {
			return domain.NewNotFoundError("schema", pathPattern)
		}

		var m schemaMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("unmarshal schema: %w", err)
		}

		attachment = schemaMetaToDomain(&m, namespace, pathPattern)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get schema: %w", err)
	}

	return attachment, nil
}

func (r *SchemaRepo) List(_ context.Context, namespace string) ([]*domain.SchemaAttachment, error) {
	var attachments []*domain.SchemaAttachment

	err := r.store.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketSchemas))
		prefix := schemaKeyPrefix(namespace)
		c := b.Cursor()

		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			_, pathPattern, _ := bytes.Cut(k, []byte{keySep})

			var m schemaMeta
			if err := json.Unmarshal(v, &m); err != nil {
				return fmt.Errorf("unmarshal schema %s: %w", k, err)
			}

			attachments = append(attachments, schemaMetaToDomain(&m, namespace, string(pathPattern)))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}

	return attachments, nil
}
