package bbolt

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

const errUnmarshalUser = "unmarshal user: %w"

// UserRepo stores and retrieves auth users in bbolt.
type UserRepo struct {
	store *Store
}

// NewUserRepo creates a new UserRepo backed by the given Store.
func NewUserRepo(store *Store) *UserRepo {
	return &UserRepo{store: store}
}

// Upsert creates or updates a user. It is called on every OIDC login.
// When the user already exists, only Name, Picture, and LastLoginAt are updated.
func (r *UserRepo) Upsert(_ context.Context, user *domain.User) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthUsers))
		key := []byte(user.Email)

		existing := b.Get(key)
		if existing == nil {
			// New user — set CreatedAt now.
			user.CreatedAt = time.Now()
		} else {
			// Existing user — preserve CreatedAt from storage.
			var m authUserMeta
			if err := json.Unmarshal(existing, &m); err != nil {
				return fmt.Errorf(errUnmarshalUser, err)
			}

			user.CreatedAt = m.CreatedAt
		}

		data, err := json.Marshal(domainToAuthUserMeta(user))
		if err != nil {
			return fmt.Errorf("marshal user: %w", err)
		}

		return b.Put(key, data)
	})
	if err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}

	return nil
}

// Get returns the user with the given email.
// Returns domain.ErrNotFound if no such user exists.
func (r *UserRepo) Get(_ context.Context, email string) (*domain.User, error) {
	var user *domain.User

	err := r.store.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthUsers))
		data := b.Get([]byte(email))

		if data == nil {
			return domain.NewNotFoundError("user", email)
		}

		var m authUserMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf(errUnmarshalUser, err)
		}

		user = authUserMetaToDomain(&m)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	return user, nil
}

// List returns all users sorted by email (bbolt ForEach iterates keys in byte order).
func (r *UserRepo) List(_ context.Context) ([]*domain.User, error) {
	var users []*domain.User

	err := r.store.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthUsers))

		return b.ForEach(func(_, v []byte) error {
			var m authUserMeta
			if err := json.Unmarshal(v, &m); err != nil {
				return fmt.Errorf(errUnmarshalUser, err)
			}

			users = append(users, authUserMetaToDomain(&m))

			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	return users, nil
}
