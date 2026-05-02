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
// Also writes a secondary index entry (id → hash) for O(1) lookup by ID.
func (r *PATRepo) Create(_ context.Context, pat *domain.PAT) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthTokens))

		data, err := json.Marshal(domainToAuthTokenMeta(pat))
		if err != nil {
			return fmt.Errorf("marshal token: %w", err)
		}

		if err = b.Put([]byte(pat.TokenHash), data); err != nil {
			return fmt.Errorf("put token: %w", err)
		}

		idx := tx.Bucket([]byte(bucketAuthTokenByID))

		return idx.Put([]byte(pat.ID), []byte(pat.TokenHash))
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

		m, err := authTokenMetaFromBytes(data)
		if err != nil {
			return err
		}

		pat = authTokenMetaToDomain(m)

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
			m, err := authTokenMetaFromBytes(v)
			if err != nil {
				return err
			}

			if userEmail != "" && m.UserEmail != userEmail {
				return nil
			}

			tokens = append(tokens, authTokenMetaToDomain(m))

			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}

	return tokens, nil
}

// GetByID returns the PAT identified by its ID using the secondary index.
// Returns domain.ErrNotFound if no such token exists.
func (r *PATRepo) GetByID(_ context.Context, id string) (*domain.PAT, error) {
	var pat *domain.PAT

	err := r.store.db.View(func(tx *bolt.Tx) error {
		idx := tx.Bucket([]byte(bucketAuthTokenByID))
		hashBytes := idx.Get([]byte(id))

		if hashBytes == nil {
			return domain.NewNotFoundError("token", id)
		}

		b := tx.Bucket([]byte(bucketAuthTokens))
		data := b.Get(hashBytes)

		if data == nil {
			return domain.NewNotFoundError("token", id)
		}

		m, err := authTokenMetaFromBytes(data)
		if err != nil {
			return err
		}

		pat = authTokenMetaToDomain(m)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get token by id: %w", err)
	}

	return pat, nil
}

// Delete removes the PAT with the given ID using the secondary index.
// Returns domain.ErrNotFound if no token with that ID exists.
func (r *PATRepo) Delete(_ context.Context, id string) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		idx := tx.Bucket([]byte(bucketAuthTokenByID))
		hashBytes := idx.Get([]byte(id))

		if hashBytes == nil {
			return domain.NewNotFoundError("token", id)
		}

		// Copy hash before deleting from the index bucket.
		tokenHash := make([]byte, len(hashBytes))
		copy(tokenHash, hashBytes)

		if err := idx.Delete([]byte(id)); err != nil {
			return fmt.Errorf("delete token id index: %w", err)
		}

		b := tx.Bucket([]byte(bucketAuthTokens))

		return b.Delete(tokenHash)
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

		m, err := authTokenMetaFromBytes(data)
		if err != nil {
			return err
		}

		m.LastUsedAt = &at
		m.LastUsedIP = ip

		newData, err := json.Marshal(m)
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
