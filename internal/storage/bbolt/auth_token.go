package bbolt

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// TokenRepo stores and retrieves service credentials (Tokens) in bbolt.
// Tokens are keyed by their SHA-256 hash for O(1) lookup during authentication.
type TokenRepo struct {
	manager *Manager
}

// NewTokenRepo creates a new TokenRepo backed by the given Manager.
func NewTokenRepo(manager *Manager) *TokenRepo {
	return &TokenRepo{manager: manager}
}

// Create stores a new Token. The caller must set all fields including TokenHash.
// Also writes a secondary index entry (id → hash) for O(1) lookup by ID.
func (r *TokenRepo) Create(ctx context.Context, token *domain.Token) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthTokens))
		// ...

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
func (r *TokenRepo) GetByHash(ctx context.Context, tokenHash string) (*domain.Token, error) {
	var token *domain.Token

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
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

// List returns tokens matching filter (namespace-scope intersect, optional
// IssuedBy, optional Name search), sorted and paginated per params.
//
// Currently full-scan over the tokens bucket — secondary indexes by namespace
// are deferred (EL-4 §12 pt 7). When token inventory grows past the point of
// noticeable list latency, introduce a `tokens_by_namespace` index.
func (r *TokenRepo) List(
	ctx context.Context,
	filter domain.TokenFilter,
	params domain.TokenListParams,
) ([]*domain.Token, int, error) {
	var matches []*domain.Token

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthTokens))

		return b.ForEach(func(_, v []byte) error {
			m, err := authTokenMetaFromBytes(v)
			if err != nil {
				return err
			}

			if len(filter.IssuedBy) > 0 && !slices.Contains(filter.IssuedBy, m.IssuedBy) {
				return nil
			}

			tok := authTokenMetaToDomain(m)

			if !tokenNamespaceMatches(tok, filter) {
				return nil
			}

			if !tokenExplicitNamespaceMatches(tok, filter.Namespaces) {
				return nil
			}

			if !tokenQueryMatches(tok.Name, filter.QueryParams) {
				return nil
			}

			matches = append(matches, tok)

			return nil
		})
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list tokens: %w", err)
	}

	sortTokens(matches, params.Sort)
	total := len(matches)
	paginated := paginateTokens(matches, params.Offset, params.Limit)

	return paginated, total, nil
}

// ListAll returns every token without filter / pagination. Convenience for
// callers that need the global view (e.g. token-by-hash lookup at auth time
// would use GetByHash, not this).
func (r *TokenRepo) ListAll(ctx context.Context) ([]*domain.Token, error) {
	tokens, _, err := r.List(ctx, domain.TokenFilter{AnyNamespace: true}, domain.TokenListParams{})
	if err != nil {
		return nil, err
	}

	return tokens, nil
}

func tokenNamespaceMatches(t *domain.Token, filter domain.TokenFilter) bool {
	if filter.AnyNamespace {
		return true
	}

	for _, ns := range t.Namespaces {
		if _, ok := filter.NamespaceScopes[ns]; ok {
			return true
		}
	}

	return false
}

func tokenQueryMatches(name string, queries []string) bool {
	if len(queries) == 0 {
		return true
	}

	lower := strings.ToLower(name)
	for _, q := range queries {
		if q == "" {
			continue
		}

		if strings.Contains(lower, strings.ToLower(q)) {
			return true
		}
	}

	return false
}

func tokenExplicitNamespaceMatches(t *domain.Token, namespaces []string) bool {
	if len(namespaces) == 0 {
		return true
	}

	for _, ns := range t.Namespaces {
		if slices.Contains(namespaces, ns) {
			return true
		}
	}

	return false
}

func sortTokens(tokens []*domain.Token, params domain.SortParams) {
	sort.Slice(tokens, func(i, j int) bool {
		a, b := tokens[i], tokens[j]

		var less bool

		switch params.Field {
		case "name":
			less = a.Name < b.Name
		case "last_used":
			less = lastUsedBefore(a, b)
		default: // "created" or empty — most recent first
			less = a.CreatedAt.After(b.CreatedAt)
		}

		if params.Desc {
			return !less
		}

		return less
	})
}

func lastUsedBefore(a, b *domain.Token) bool {
	switch {
	case a.LastUsedAt == nil && b.LastUsedAt == nil:
		return a.CreatedAt.Before(b.CreatedAt)
	case a.LastUsedAt == nil:
		return true
	case b.LastUsedAt == nil:
		return false
	default:
		return a.LastUsedAt.Before(*b.LastUsedAt)
	}
}

func paginateTokens(tokens []*domain.Token, offset, limit int) []*domain.Token {
	if offset < 0 {
		offset = 0
	}

	if offset >= len(tokens) {
		return []*domain.Token{}
	}

	end := len(tokens)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}

	return tokens[offset:end]
}

// GetByID returns the Token identified by its ID using the secondary index.
// Returns domain.ErrNotFound if no such token exists.
func (r *TokenRepo) GetByID(ctx context.Context, id string) (*domain.Token, error) {
	var token *domain.Token

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
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
func (r *TokenRepo) Delete(ctx context.Context, id string) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
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
func (r *TokenRepo) UpdateLastUsed(ctx context.Context, tokenHash, ip string, at time.Time) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
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
