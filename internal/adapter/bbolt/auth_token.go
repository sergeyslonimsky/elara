package bbolt

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// PATRepo stores and retrieves Personal Access Tokens in bbolt.
// Tokens are keyed by their SHA-256 hash for O(1) lookup during authentication.
type PATRepo struct {
	store *Store
}

// NewPATRepo creates a new PATRepo backed by the given Store.
func NewPATRepo(store *Store) *PATRepo {
	return &PATRepo{store: store}
}

// Create stores a new PAT. The caller must set all fields including TokenHash.
func (r *PATRepo) Create(_ context.Context, pat *domain.PAT) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthTokens))

		data, err := json.Marshal(domainToAuthTokenMeta(pat))
		if err != nil {
			return fmt.Errorf("marshal token: %w", err)
		}

		return b.Put([]byte(pat.TokenHash), data)
	})
	if err != nil {
		return fmt.Errorf("create token: %w", err)
	}

	return nil
}

// GetByHash returns the PAT identified by its SHA-256 hex hash.
// Returns domain.ErrNotFound if no such token exists.
func (r *PATRepo) GetByHash(_ context.Context, tokenHash string) (*domain.PAT, error) {
	var pat *domain.PAT

	err := r.store.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthTokens))
		data := b.Get([]byte(tokenHash))

		if data == nil {
			return domain.NewNotFoundError("token", tokenHash)
		}

		var m authTokenMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("unmarshal token: %w", err)
		}

		pat = authTokenMetaToDomain(&m)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get token by hash: %w", err)
	}

	return pat, nil
}

// List returns PATs filtered by userEmail. An empty userEmail returns all tokens (admin view).
func (r *PATRepo) List(_ context.Context, userEmail string) ([]*domain.PAT, error) {
	var tokens []*domain.PAT

	err := r.store.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthTokens))

		return b.ForEach(func(_, v []byte) error {
			var m authTokenMeta
			if err := json.Unmarshal(v, &m); err != nil {
				return fmt.Errorf("unmarshal token: %w", err)
			}

			if userEmail != "" && m.UserEmail != userEmail {
				return nil
			}

			tokens = append(tokens, authTokenMetaToDomain(&m))

			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}

	return tokens, nil
}

// Delete removes the PAT with the given ID. Iterates all tokens to find it.
// Returns domain.ErrNotFound if no token with that ID exists.
func (r *PATRepo) Delete(_ context.Context, id string) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthTokens))

		var foundKey []byte

		err := b.ForEach(func(k, v []byte) error {
			var m authTokenMeta
			if err := json.Unmarshal(v, &m); err != nil {
				return fmt.Errorf("unmarshal token: %w", err)
			}

			if m.ID == id {
				// Copy the key because it is only valid inside the transaction.
				foundKey = make([]byte, len(k))
				copy(foundKey, k)
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("iterate tokens: %w", err)
		}

		if foundKey == nil {
			return domain.NewNotFoundError("token", id)
		}

		return b.Delete(foundKey)
	})
	if err != nil {
		return fmt.Errorf("delete token: %w", err)
	}

	return nil
}

// UpdateLastUsed updates the LastUsedAt and LastUsedIP fields of a token identified by its hash.
func (r *PATRepo) UpdateLastUsed(_ context.Context, tokenHash, ip string, at time.Time) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthTokens))
		key := []byte(tokenHash)
		data := b.Get(key)

		if data == nil {
			return domain.NewNotFoundError("token", tokenHash)
		}

		var m authTokenMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("unmarshal token: %w", err)
		}

		m.LastUsedAt = &at
		m.LastUsedIP = ip

		newData, err := json.Marshal(&m)
		if err != nil {
			return fmt.Errorf("marshal token: %w", err)
		}

		return b.Put(key, newData)
	})
	if err != nil {
		return fmt.Errorf("update token last used: %w", err)
	}

	return nil
}
