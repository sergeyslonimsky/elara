package bbolt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

// GroupRepo stores and retrieves auth groups in bbolt.
type GroupRepo struct {
	store *Store
	tx    storage.Tx
}

// NewGroupRepo creates a new GroupRepo backed by the given Store.
func NewGroupRepo(store *Store) *GroupRepo {
	return &GroupRepo{store: store}
}

// WithTx returns a new GroupRepo that uses the provided transaction.
func (r *GroupRepo) WithTx(tx storage.Tx) *GroupRepo {
	return &GroupRepo{
		store: r.store,
		tx:    tx,
	}
}

func (r *GroupRepo) view(fn func(storage.Tx) error) error {
	if r.tx != nil {
		return fn(r.tx)
	}

	return r.store.db.View(func(tx *bolt.Tx) error {
		return fn(&txWrapper{tx: tx})
	})
}

func (r *GroupRepo) update(fn func(storage.Tx) error) error {
	if r.tx != nil {
		return fn(r.tx)
	}

	return r.store.db.Update(func(tx *bolt.Tx) error {
		return fn(&txWrapper{tx: tx})
	})
}

// Create stores a new group. Returns domain.ErrAlreadyExists if the ID is already taken.
func (r *GroupRepo) Create(_ context.Context, group *domain.Group) error {
	err := r.update(func(tx storage.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))
		key := []byte(group.ID)

		if b.Get(key) != nil {
			return domain.NewAlreadyExistsError("group", group.ID)
		}

		now := time.Now()
		group.CreatedAt = now
		group.UpdatedAt = now
		if group.Version == 0 {
			group.Version = 1
		}

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

	err := r.view(func(tx storage.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))
		data := b.Get([]byte(id))

		if data == nil {
			return domain.NewNotFoundError("group", id)
		}

		m, err := authGroupMetaFromBytes(data)
		if err != nil {
			return err
		}

		group = authGroupMetaToDomain(m)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get group: %w", err)
	}

	return group, nil
}

// Update replaces a group's Name and Members. Returns domain.ErrNotFound if missing.
func (r *GroupRepo) Update(_ context.Context, group *domain.Group) error {
	err := r.update(func(tx storage.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))
		key := []byte(group.ID)
		data := b.Get(key)

		if data == nil {
			return domain.NewNotFoundError("group", group.ID)
		}

		existing, err := authGroupMetaFromBytes(data)
		if err != nil {
			return err
		}

		existing.Name = group.Name
		existing.Description = group.Description
		existing.Members = group.Members
		existing.Version = group.Version
		existing.UpdatedAt = time.Now()

		group.CreatedAt = existing.CreatedAt
		group.UpdatedAt = existing.UpdatedAt
		group.System = existing.System

		newData, err := json.Marshal(existing)
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
	err := r.update(func(tx storage.Tx) error {
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

var errFound = errors.New("found") // sentinel for early ForEach exit

// FindByName returns the first group with the given name.
// Returns domain.ErrNotFound if no group has that name.
func (r *GroupRepo) FindByName(_ context.Context, name string) (*domain.Group, error) {
	var found *domain.Group

	err := r.view(func(tx storage.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))

		return b.ForEach(func(_, v []byte) error {
			m, err := authGroupMetaFromBytes(v)
			if err != nil {
				return err
			}

			if m.Name == name {
				found = authGroupMetaToDomain(m)

				return errFound
			}

			return nil
		})
	})
	if err != nil && !errors.Is(err, errFound) {
		return nil, fmt.Errorf("find group by name: %w", err)
	}

	if found == nil {
		return nil, fmt.Errorf("find group by name: %w", domain.NewNotFoundError("group", name))
	}

	return found, nil
}

// List returns all groups sorted by ID (bbolt ForEach iterates keys in byte order).
func (r *GroupRepo) List(_ context.Context) ([]*domain.Group, error) {
	var groups []*domain.Group

	err := r.view(func(tx storage.Tx) error {
		b := tx.Bucket([]byte(bucketAuthGroups))

		return b.ForEach(func(_, v []byte) error {
			m, err := authGroupMetaFromBytes(v)
			if err != nil {
				return err
			}

			groups = append(groups, authGroupMetaToDomain(m))

			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	return groups, nil
}
