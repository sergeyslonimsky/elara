package bbolt

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// GroupRepo stores and retrieves auth groups in bbolt.
type GroupRepo struct {
	store *Store
}

// NewGroupRepo creates a new GroupRepo backed by the given Store.
func NewGroupRepo(store *Store) *GroupRepo {
	return &GroupRepo{store: store}
}

// Create stores a new group. Returns domain.ErrAlreadyExists if the ID is already taken.
func (r *GroupRepo) Create(_ context.Context, group *domain.Group) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))
		key := []byte(group.ID)

		if b.Get(key) != nil {
			return domain.NewAlreadyExistsError("group", group.ID)
		}

		now := time.Now()
		group.CreatedAt = now
		group.UpdatedAt = now

		data, err := json.Marshal(domainToAuthGroupMeta(group))
		if err != nil {
			return fmt.Errorf("marshal group: %w", err)
		}

		return b.Put(key, data)
	})
	if err != nil {
		return fmt.Errorf("create group: %w", err)
	}

	return nil
}

// Get returns the group with the given ID. Returns domain.ErrNotFound if missing.
func (r *GroupRepo) Get(_ context.Context, id string) (*domain.Group, error) {
	var group *domain.Group

	err := r.store.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))
		data := b.Get([]byte(id))

		if data == nil {
			return domain.NewNotFoundError("group", id)
		}

		var m authGroupMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("unmarshal group: %w", err)
		}

		group = authGroupMetaToDomain(&m)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get group: %w", err)
	}

	return group, nil
}

// Update replaces a group's Name and Members. Returns domain.ErrNotFound if missing.
func (r *GroupRepo) Update(_ context.Context, group *domain.Group) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))
		key := []byte(group.ID)
		data := b.Get(key)

		if data == nil {
			return domain.NewNotFoundError("group", group.ID)
		}

		var existing authGroupMeta
		if err := json.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("unmarshal group: %w", err)
		}

		existing.Name = group.Name
		existing.Members = group.Members
		existing.UpdatedAt = time.Now()

		group.CreatedAt = existing.CreatedAt
		group.UpdatedAt = existing.UpdatedAt

		newData, err := json.Marshal(&existing)
		if err != nil {
			return fmt.Errorf("marshal group: %w", err)
		}

		return b.Put(key, newData)
	})
	if err != nil {
		return fmt.Errorf("update group: %w", err)
	}

	return nil
}

// Delete removes the group with the given ID. Returns domain.ErrNotFound if missing.
func (r *GroupRepo) Delete(_ context.Context, id string) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))
		key := []byte(id)

		if b.Get(key) == nil {
			return domain.NewNotFoundError("group", id)
		}

		return b.Delete(key)
	})
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	return nil
}

// List returns all groups sorted by ID (bbolt ForEach iterates keys in byte order).
func (r *GroupRepo) List(_ context.Context) ([]*domain.Group, error) {
	var groups []*domain.Group

	err := r.store.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))

		return b.ForEach(func(_, v []byte) error {
			var m authGroupMeta
			if err := json.Unmarshal(v, &m); err != nil {
				return fmt.Errorf("unmarshal group: %w", err)
			}

			groups = append(groups, authGroupMetaToDomain(&m))

			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	return groups, nil
}
