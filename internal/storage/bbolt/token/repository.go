// Package token is the bbolt-backed repository for domain.Token (service
// credentials for the etcd-compatible gRPC API).
//
// Repository contract:
//   - Pure CRUD (Create, GetByHash, GetByID, Delete) is "dumb": callers
//     prepare every field, the repo writes verbatim.
//   - A secondary index (`auth_token_by_id`) maps ID → TokenHash for O(1)
//     lookup-by-ID. Create and Delete keep it in sync; there is no
//     repair pass.
//   - UpdateLastUsed performs a read-modify-write and wraps its own
//     WithTx so the load+store is atomic even when called outside a
//     usecase-owned transaction.
//   - List applies search/sort/pagination — that is data shaping, not
//     business validation.
package token

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/storage/internal"
	"github.com/sergeyslonimsky/elara/pkg/bbolt"
)

type Repository struct {
	dbm bbolt.Manager
}

func NewRepository(dbm bbolt.Manager) *Repository {
	return &Repository{dbm: dbm}
}

// Create persists a brand-new token and writes its secondary-index entries.
// Returns storage.ErrResourceAlreadyExists if a token with the same hash
// already lives in the primary bucket, and domain.ErrTokenUsernameTaken if
// another token already holds token.Username.
func (r *Repository) Create(ctx context.Context, token *domain.Token) error {
	q := r.dbm.GetQuerier(ctx)

	// Checked before any write: outside a transaction a late failure would
	// leave the primary and id-index records behind.
	if token.Username == "" {
		return fmt.Errorf("token %s: %w", token.ID, domain.NewValidationError("username", "username is required"))
	}

	if bbolt.Exists(q, bucketTokens, []byte(token.TokenHash)) {
		return fmt.Errorf("token %s: %w", token.ID, storage.ErrResourceAlreadyExists)
	}

	usernameIdx := q.Bucket(bucketTokensByUsername)
	if usernameIdx.Get([]byte(token.Username)) != nil {
		return fmt.Errorf("token username %q: %w", token.Username, domain.ErrTokenUsernameTaken)
	}

	if err := bbolt.Put(q, bucketTokens, []byte(token.TokenHash), internal.DomainToTokenMeta(token)); err != nil {
		return fmt.Errorf("put token: %w", err)
	}

	if err := q.Bucket(bucketTokensByID).Put([]byte(token.ID), []byte(token.TokenHash)); err != nil {
		return fmt.Errorf("put token id index: %w", err)
	}

	if err := usernameIdx.Put([]byte(token.Username), []byte(token.TokenHash)); err != nil {
		return fmt.Errorf("put token username index: %w", err)
	}

	return nil
}

// GetByHash returns the token identified by its SHA-256 hex hash.
func (r *Repository) GetByHash(ctx context.Context, tokenHash string) (*domain.Token, error) {
	m, err := bbolt.Get[internal.TokenMeta](r.dbm.GetQuerier(ctx), bucketTokens, []byte(tokenHash))
	if errors.Is(err, bbolt.ErrNotFound) {
		return nil, fmt.Errorf("token %s: %w", tokenHash, storage.ErrResourceNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get token by hash: %w", err)
	}

	return internal.TokenMetaToDomain(m), nil
}

// GetByUsername returns the token whose Username matches exactly.
func (r *Repository) GetByUsername(ctx context.Context, username string) (*domain.Token, error) {
	q := r.dbm.GetQuerier(ctx)

	hashBytes := q.Bucket(bucketTokensByUsername).Get([]byte(username))
	if hashBytes == nil {
		return nil, fmt.Errorf("token username %q: %w", username, storage.ErrResourceNotFound)
	}

	m, err := bbolt.Get[internal.TokenMeta](q, bucketTokens, hashBytes)
	if errors.Is(err, bbolt.ErrNotFound) {
		return nil, fmt.Errorf("token username %q: %w", username, storage.ErrResourceNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get token by username: %w", err)
	}

	return internal.TokenMetaToDomain(m), nil
}

// SetUsername assigns a Username to a token that has none yet (tokens
// created before Username existed). It refuses to overwrite an existing
// Username — the login is immutable once handed out — and returns
// domain.ErrTokenUsernameTaken if another token already holds username.
func (r *Repository) SetUsername(ctx context.Context, id, username string) error {
	q := r.dbm.GetQuerier(ctx)

	hashBytes := q.Bucket(bucketTokensByID).Get([]byte(id))
	if hashBytes == nil {
		return fmt.Errorf("token %s: %w", id, storage.ErrResourceNotFound)
	}

	tokenHash := make([]byte, len(hashBytes))
	copy(tokenHash, hashBytes)

	m, err := bbolt.Get[internal.TokenMeta](q, bucketTokens, tokenHash)
	if err != nil {
		return fmt.Errorf("get token %s: %w", id, err)
	}

	if m.Username != "" {
		return fmt.Errorf("token %s already has a username: %w", id, domain.ErrCanonicalNameImmutable)
	}

	usernameIdx := q.Bucket(bucketTokensByUsername)
	if usernameIdx.Get([]byte(username)) != nil {
		return fmt.Errorf("token username %q: %w", username, domain.ErrTokenUsernameTaken)
	}

	m.Username = username

	if err := bbolt.Put(q, bucketTokens, tokenHash, m); err != nil {
		return fmt.Errorf("put token: %w", err)
	}

	if err := usernameIdx.Put([]byte(username), tokenHash); err != nil {
		return fmt.Errorf("put token username index: %w", err)
	}

	return nil
}

// GetByID returns the token identified by its ID using the secondary index.
func (r *Repository) GetByID(ctx context.Context, id string) (*domain.Token, error) {
	q := r.dbm.GetQuerier(ctx)

	hashBytes := q.Bucket(bucketTokensByID).Get([]byte(id))
	if hashBytes == nil {
		return nil, fmt.Errorf("token %s: %w", id, storage.ErrResourceNotFound)
	}

	m, err := bbolt.Get[internal.TokenMeta](q, bucketTokens, hashBytes)
	if errors.Is(err, bbolt.ErrNotFound) {
		return nil, fmt.Errorf("token %s: %w", id, storage.ErrResourceNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get token by id: %w", err)
	}

	return internal.TokenMetaToDomain(m), nil
}

// Delete removes the token with the given ID together with its index entry.
func (r *Repository) Delete(ctx context.Context, id string) error {
	q := r.dbm.GetQuerier(ctx)

	idxBucket := q.Bucket(bucketTokensByID)
	hashBytes := idxBucket.Get([]byte(id))
	if hashBytes == nil {
		return fmt.Errorf("token %s: %w", id, storage.ErrResourceNotFound)
	}

	// Copy hash before deleting from the index bucket — bbolt may reuse
	// the underlying memory once the entry is removed.
	tokenHash := make([]byte, len(hashBytes))
	copy(tokenHash, hashBytes)

	m, err := bbolt.Get[internal.TokenMeta](q, bucketTokens, tokenHash)
	if err != nil && !errors.Is(err, bbolt.ErrNotFound) {
		return fmt.Errorf("get token %s: %w", id, err)
	}

	if m.Username != "" {
		if err := q.Bucket(bucketTokensByUsername).Delete([]byte(m.Username)); err != nil {
			return fmt.Errorf("delete token username index: %w", err)
		}
	}

	if err := idxBucket.Delete([]byte(id)); err != nil {
		return fmt.Errorf("delete token id index: %w", err)
	}

	if err := bbolt.Delete(q, bucketTokens, tokenHash); err != nil {
		return fmt.Errorf("delete token: %w", err)
	}

	return nil
}

// UpdateLastUsed sets LastUsedAt/LastUsedIP on the token identified by hash.
//
// Atomicity of the read-modify-write is the caller's responsibility — wrap
// in Manager.WithTx when concurrent writers may touch the same token.
func (r *Repository) UpdateLastUsed(ctx context.Context, tokenHash, ip string, at time.Time) error {
	q := r.dbm.GetQuerier(ctx)

	m, err := bbolt.Get[internal.TokenMeta](q, bucketTokens, []byte(tokenHash))
	if errors.Is(err, bbolt.ErrNotFound) {
		return fmt.Errorf("token %s: %w", tokenHash, storage.ErrResourceNotFound)
	}
	if err != nil {
		return fmt.Errorf("update token last used: %w", err)
	}

	m.LastUsedAt = &at
	m.LastUsedIP = ip

	if err := bbolt.Put(q, bucketTokens, []byte(tokenHash), m); err != nil {
		return fmt.Errorf("update token last used: %w", err)
	}

	return nil
}

// List returns tokens matching filter, applies sort and pagination. Total is
// the count after filtering but before pagination so callers can render page
// indicators. Currently a full-scan over the tokens bucket — secondary
// indexes by namespace are deferred until token inventory grows.
func (r *Repository) List(
	ctx context.Context,
	filter domain.TokenFilter,
	params domain.TokenListParams,
) ([]*domain.Token, int, error) {
	var matches []*domain.Token

	bucket := r.dbm.GetQuerier(ctx).Bucket(bucketTokens)
	codec := bbolt.JSONCodec[internal.TokenMeta]{}

	err := bucket.ForEach(func(_, v []byte) error {
		var m internal.TokenMeta
		if err := codec.Unmarshal(v, &m); err != nil {
			return fmt.Errorf("decode token: %w", err)
		}

		if len(filter.IssuedBy) > 0 && !slices.Contains(filter.IssuedBy, m.IssuedBy) {
			return nil
		}

		tok := internal.TokenMetaToDomain(m)

		if !namespaceMatches(tok, filter) {
			return nil
		}

		if !explicitNamespaceMatches(tok, filter.Namespaces) {
			return nil
		}

		if !queryMatches(filter.QueryParams, tok.Name, tok.Username) {
			return nil
		}

		matches = append(matches, tok)

		return nil
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list tokens: %w", err)
	}

	sortTokens(matches, params.Sort)
	total := len(matches)
	paginated := paginate(matches, params.Offset, params.Limit)

	return paginated, total, nil
}

// ListAll returns every token without filter / pagination.
func (r *Repository) ListAll(ctx context.Context) ([]*domain.Token, error) {
	tokens, _, err := r.List(ctx, domain.TokenFilter{AnyNamespace: true}, domain.TokenListParams{})
	if err != nil {
		return nil, err
	}

	return tokens, nil
}

func namespaceMatches(t *domain.Token, filter domain.TokenFilter) bool {
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

func explicitNamespaceMatches(t *domain.Token, namespaces []string) bool {
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

// queryMatches reports whether any query is a case-insensitive substring of
// any of fields — a token matches by its name prefix or its full username.
func queryMatches(queries []string, fields ...string) bool {
	if len(queries) == 0 {
		return true
	}

	for _, q := range queries {
		if q == "" {
			continue
		}

		lq := strings.ToLower(q)
		for _, f := range fields {
			if strings.Contains(strings.ToLower(f), lq) {
				return true
			}
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
		case "username":
			less = a.Username < b.Username
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

func paginate(tokens []*domain.Token, offset, limit int) []*domain.Token {
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
