package bbolt

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// TokenRepo stores and retrieves service credentials (Tokens) in bbolt.
// Tokens are keyed by their SHA-256 hash for O(1) lookup during authentication.
type TokenRepo struct {
	store *Store
}

// NewTokenRepo creates a new TokenRepo backed by the given Store.
func NewTokenRepo(store *Store) *TokenRepo {
	return &TokenRepo{store: store}
}

// Create stores a new Token. The caller must set all fields including TokenHash.
// Also writes a secondary index entry (id → hash) for O(1) lookup by ID.
func (r *TokenRepo) Create(_ context.Context, token *domain.Token) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthTokens))

		data, err := json.Marshal(domainToAuthTokenMeta(token))
		if err != nil {
			return fmt.Errorf("marshal token: %w", err)
		}

		if err = b.Put([]byte(token.TokenHash), data); err != nil {
			return fmt.Errorf("put token: %w", err)
		}

		idx := tx.Bucket([]byte(bucketAuthTokenByID))

		return idx.Put([]byte(token.ID), []byte(token.TokenHash))
	})
	if err != nil {
		return fmt.Errorf("create token: %w", err)
	}

	return nil
}

// GetByHash returns the Token identified by its SHA-256 hex hash.
// Returns domain.ErrNotFound if no such token exists.
func (r *TokenRepo) GetByHash(_ context.Context, tokenHash string) (*domain.Token, error) {
	var token *domain.Token

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

		token = authTokenMetaToDomain(m)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get token by hash: %w", err)
	}

	return token, nil
}

// List returns Tokens filtered by issuedBy. An empty issuedBy returns all tokens (admin view).
func (r *TokenRepo) List(_ context.Context, issuedBy string) ([]*domain.Token, error) {
	var tokens []*domain.Token

	err := r.store.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthTokens))

		return b.ForEach(func(_, v []byte) error {
			m, err := authTokenMetaFromBytes(v)
			if err != nil {
				return err
			}

			if issuedBy != "" && m.IssuedBy != issuedBy {
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

// GetByID returns the Token identified by its ID using the secondary index.
// Returns domain.ErrNotFound if no such token exists.
func (r *TokenRepo) GetByID(_ context.Context, id string) (*domain.Token, error) {
	var token *domain.Token

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

		token = authTokenMetaToDomain(m)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get token by id: %w", err)
	}

	return token, nil
}

// Delete removes the Token with the given ID using the secondary index.
// Returns domain.ErrNotFound if no token with that ID exists.
func (r *TokenRepo) Delete(_ context.Context, id string) error {
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
func (r *TokenRepo) UpdateLastUsed(_ context.Context, tokenHash, ip string, at time.Time) error {
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
