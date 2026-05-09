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

type NamespaceRepo struct {
	store *Store
}

func NewNamespaceRepo(store *Store) *NamespaceRepo {
	return &NamespaceRepo{store: store}
}

func (r *NamespaceRepo) Create(_ context.Context, ns *domain.Namespace) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNamespaces))
		key := []byte(ns.Name)

		if b.Get(key) != nil {
			return domain.NewAlreadyExistsError("namespace", ns.Name)
		}

		now := time.Now()
		ns.CreatedAt = now
		ns.UpdatedAt = now

		data, err := json.Marshal(domainToNamespaceMeta(ns))
		if err != nil {
			return fmt.Errorf("marshal namespace: %w", err)
		}

		return b.Put(key, data)
	})
	if err != nil {
		return fmt.Errorf("create namespace: %w", err)
	}

	return nil
}

func (r *NamespaceRepo) Get(_ context.Context, name string) (*domain.Namespace, error) {
	var ns *domain.Namespace

	err := r.store.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNamespaces))
		data := b.Get([]byte(name))

		if data == nil {
			return domain.NewNotFoundError("namespace", name)
		}

		var m namespaceMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("unmarshal namespace: %w", err)
		}

		ns = namespaceMetaToDomain(&m, name)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get namespace: %w", err)
	}

	return ns, nil
}

func (r *NamespaceRepo) List(_ context.Context) ([]*domain.Namespace, error) {
	var namespaces []*domain.Namespace

	err := r.store.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNamespaces))

		return b.ForEach(func(k, v []byte) error {
			var m namespaceMeta
			if err := json.Unmarshal(v, &m); err != nil {
				return fmt.Errorf("unmarshal namespace %s: %w", k, err)
			}

			namespaces = append(namespaces, namespaceMetaToDomain(&m, string(k)))

			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	return namespaces, nil
}

func (r *NamespaceRepo) Update(_ context.Context, ns *domain.Namespace) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNamespaces))
		data := b.Get([]byte(ns.Name))

		if data == nil {
			return domain.NewNotFoundError("namespace", ns.Name)
		}

		var existing namespaceMeta
		if err := json.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("unmarshal namespace: %w", err)
		}

		if existing.Locked {
			return fmt.Errorf("namespace %q: %w", ns.Name, domain.ErrNamespaceLocked)
		}

		existing.Description = ns.Description
		existing.UpdatedAt = time.Now()

		newData, err := json.Marshal(&existing)
		if err != nil {
			return fmt.Errorf("marshal namespace: %w", err)
		}

		if err := b.Put([]byte(ns.Name), newData); err != nil {
			return fmt.Errorf("put namespace: %w", err)
		}

		ns.CreatedAt = existing.CreatedAt
		ns.UpdatedAt = existing.UpdatedAt

		return nil
	})
	if err != nil {
		return fmt.Errorf("update namespace: %w", err)
	}

	return nil
}

func (r *NamespaceRepo) Delete(_ context.Context, name string) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNamespaces))
		data := b.Get([]byte(name))

		if data == nil {
			return domain.NewNotFoundError("namespace", name)
		}

		var m namespaceMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("unmarshal namespace: %w", err)
		}

		if m.Locked {
			return fmt.Errorf("namespace %q: %w", name, domain.ErrNamespaceLocked)
		}

		return b.Delete([]byte(name))
	})
	if err != nil {
		return fmt.Errorf("delete namespace: %w", err)
	}

	return nil
}

func (r *NamespaceRepo) CountConfigs(_ context.Context, name string) (int, error) {
	var count int

	err := r.store.db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte(bucketMeta))
		prefix := configKeyPrefix(name, "")
		c := meta.Cursor()

		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			count++
		}

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("count namespace configs: %w", err)
	}

	return count, nil
}

func (r *NamespaceRepo) LockNamespace(_ context.Context, name string) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNamespaces))
		data := b.Get([]byte(name))

		if data == nil {
			return domain.NewNotFoundError("namespace", name)
		}

		var m namespaceMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("unmarshal namespace: %w", err)
		}

		if m.Locked {
			return nil
		}

		m.Locked = true

		newData, err := json.Marshal(&m)
		if err != nil {
			return fmt.Errorf("marshal namespace: %w", err)
		}

		if err := b.Put([]byte(name), newData); err != nil {
			return fmt.Errorf("put namespace: %w", err)
		}

		return writeLockHistory(tx, name, "", domain.EventTypeNamespaceLocked)
	})
	if err != nil {
		return fmt.Errorf("lock namespace: %w", err)
	}

	return nil
}

func (r *NamespaceRepo) UnlockNamespace(_ context.Context, name string) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNamespaces))
		data := b.Get([]byte(name))

		if data == nil {
			return domain.NewNotFoundError("namespace", name)
		}

		var m namespaceMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("unmarshal namespace: %w", err)
		}

		if !m.Locked {
			return nil
		}

		m.Locked = false

		newData, err := json.Marshal(&m)
		if err != nil {
			return fmt.Errorf("marshal namespace: %w", err)
		}

		if err := b.Put([]byte(name), newData); err != nil {
			return fmt.Errorf("put namespace: %w", err)
		}

		return writeLockHistory(tx, name, "", domain.EventTypeNamespaceUnlocked)
	})
	if err != nil {
		return fmt.Errorf("unlock namespace: %w", err)
	}

	return nil
}

func (r *NamespaceRepo) UpdateTimestamp(_ context.Context, name string) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketNamespaces))
		data := b.Get([]byte(name))

		if data == nil {
			return domain.NewNotFoundError("namespace", name)
		}

		var m namespaceMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("unmarshal namespace: %w", err)
		}

		m.UpdatedAt = time.Now()

		newData, err := json.Marshal(&m)
		if err != nil {
			return fmt.Errorf("marshal namespace: %w", err)
		}

		return b.Put([]byte(name), newData)
	})
	if err != nil {
		return fmt.Errorf("update namespace timestamp: %w", err)
	}

	return nil
}
