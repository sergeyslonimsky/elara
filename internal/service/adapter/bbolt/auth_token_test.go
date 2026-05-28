package bbolt_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	bboltadapter "github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
)

func newTestToken(id, issuedBy, hash string) *domain.Token {
	return &domain.Token{
		ID:         id,
		IssuedBy:   issuedBy,
		Name:       "Test Token " + id,
		TokenHash:  hash,
		Namespaces: []string{"prod"},
		Role:       "writer",
		CreatedAt:  time.Now(),
	}
}

func TestTokenRepo_Create(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewTokenRepo(store)
	ctx := t.Context()

	token := newTestToken("token-1", "alice@example.com", "hash-abc123")
	err := repo.Create(ctx, token)
	require.NoError(t, err)
}

func TestTokenRepo_GetByHash(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewTokenRepo(store)
	ctx := t.Context()

	token := newTestToken("token-2", "bob@example.com", "hash-def456")
	require.NoError(t, repo.Create(ctx, token))

	got, err := repo.GetByHash(ctx, "hash-def456")
	require.NoError(t, err)
	assert.Equal(t, "token-2", got.ID)
	assert.Equal(t, "bob@example.com", got.IssuedBy)
	assert.Equal(t, "hash-def456", got.TokenHash)
}

func TestTokenRepo_GetByHash_Missing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewTokenRepo(store)
	ctx := t.Context()

	_, err := repo.GetByHash(ctx, "nonexistent-hash")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTokenRepo_List_ByIssuedBy(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewTokenRepo(store)
	ctx := t.Context()

	require.NoError(t, repo.Create(ctx, newTestToken("token-u1", "carol@example.com", "hash-u1")))
	require.NoError(t, repo.Create(ctx, newTestToken("token-u2", "carol@example.com", "hash-u2")))
	require.NoError(t, repo.Create(ctx, newTestToken("token-u3", "dave@example.com", "hash-u3")))

	carolTokens, total, err := repo.List(ctx, domain.TokenFilter{
		AnyNamespace: true,
		IssuedBy:     []string{"carol@example.com"},
	}, domain.TokenListParams{})
	require.NoError(t, err)
	assert.Len(t, carolTokens, 2)
	assert.Equal(t, 2, total)

	daveTokens, total, err := repo.List(ctx, domain.TokenFilter{
		AnyNamespace: true,
		IssuedBy:     []string{"dave@example.com"},
	}, domain.TokenListParams{})
	require.NoError(t, err)
	assert.Len(t, daveTokens, 1)
	assert.Equal(t, 1, total)
}

func TestTokenRepo_ListAll(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewTokenRepo(store)
	ctx := t.Context()

	// Empty list.
	all, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, all)

	// Populate with tokens for different users.
	require.NoError(t, repo.Create(ctx, newTestToken("token-a1", "eve@example.com", "hash-a1")))
	require.NoError(t, repo.Create(ctx, newTestToken("token-a2", "frank@example.com", "hash-a2")))
	require.NoError(t, repo.Create(ctx, newTestToken("token-a3", "grace@example.com", "hash-a3")))

	all, err = repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestTokenRepo_List(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	earlier := now.Add(-2 * time.Hour)
	earliest := now.Add(-5 * time.Hour)

	type seedToken struct {
		id         string
		issuedBy   string
		name       string
		namespaces []string
		createdAt  time.Time
		lastUsedAt *time.Time
	}

	mkSeed := func(s seedToken) *domain.Token {
		return &domain.Token{
			ID:         s.id,
			IssuedBy:   s.issuedBy,
			Name:       s.name,
			TokenHash:  "hash-" + s.id,
			Namespaces: s.namespaces,
			Role:       "writer",
			CreatedAt:  s.createdAt,
			LastUsedAt: s.lastUsedAt,
		}
	}

	tests := []struct {
		name      string
		seeds     []seedToken
		filter    domain.TokenFilter
		params    domain.TokenListParams
		wantIDs   []string
		wantTotal int
	}{
		{
			name: "AnyNamespace returns all sorted by created desc",
			seeds: []seedToken{
				{id: "a", issuedBy: "u@x", name: "a", namespaces: []string{"ns1"}, createdAt: earliest},
				{id: "b", issuedBy: "u@x", name: "b", namespaces: []string{"ns1"}, createdAt: now},
				{id: "c", issuedBy: "u@x", name: "c", namespaces: []string{"ns1"}, createdAt: earlier},
			},
			filter:    domain.TokenFilter{AnyNamespace: true},
			params:    domain.TokenListParams{},
			wantIDs:   []string{"b", "c", "a"},
			wantTotal: 3,
		},
		{
			name: "NamespaceScopes filter returns intersection",
			seeds: []seedToken{
				{id: "a", issuedBy: "u@x", name: "a", namespaces: []string{"ns1"}, createdAt: now},
				{id: "b", issuedBy: "u@x", name: "b", namespaces: []string{"ns2"}, createdAt: now},
				{id: "c", issuedBy: "u@x", name: "c", namespaces: []string{"ns1"}, createdAt: earlier},
				{id: "d", issuedBy: "u@x", name: "d", namespaces: []string{"ns3"}, createdAt: now},
			},
			filter: domain.TokenFilter{
				NamespaceScopes: map[string]struct{}{"ns1": {}},
			},
			params:    domain.TokenListParams{Sort: domain.SortParams{Field: "name"}},
			wantIDs:   []string{"a", "c"},
			wantTotal: 2,
		},
		{
			name: "token with multiple namespaces matches if any overlaps with scope",
			seeds: []seedToken{
				{id: "a", issuedBy: "u@x", name: "a", namespaces: []string{"ns1", "ns2"}, createdAt: now},
			},
			filter: domain.TokenFilter{
				NamespaceScopes: map[string]struct{}{"ns2": {}},
			},
			wantIDs:   []string{"a"},
			wantTotal: 1,
		},
		{
			name: "token with no overlap is filtered out",
			seeds: []seedToken{
				{id: "a", issuedBy: "u@x", name: "a", namespaces: []string{"ns3"}, createdAt: now},
			},
			filter: domain.TokenFilter{
				NamespaceScopes: map[string]struct{}{"ns1": {}, "ns2": {}},
			},
			wantIDs:   []string{},
			wantTotal: 0,
		},
		{
			name: "IssuedBy filter narrows additionally",
			seeds: []seedToken{
				{id: "a", issuedBy: "alice@x", name: "a", namespaces: []string{"ns1"}, createdAt: now},
				{id: "b", issuedBy: "bob@x", name: "b", namespaces: []string{"ns1"}, createdAt: now},
			},
			filter: domain.TokenFilter{
				AnyNamespace: true,
				IssuedBy:     []string{"alice@x"},
			},
			wantIDs:   []string{"a"},
			wantTotal: 1,
		},
		{
			name: "QueryParams filter case-insensitive substring on Name",
			seeds: []seedToken{
				{id: "a", issuedBy: "u@x", name: "prod-key", namespaces: []string{"ns1"}, createdAt: now},
				{id: "b", issuedBy: "u@x", name: "stg-key", namespaces: []string{"ns1"}, createdAt: now},
			},
			filter: domain.TokenFilter{
				AnyNamespace: true,
				QueryParams:  []string{"PROD"},
			},
			wantIDs:   []string{"a"},
			wantTotal: 1,
		},
		{
			name: "pagination offset+limit slices, total preserved",
			seeds: []seedToken{
				{id: "a", issuedBy: "u@x", name: "a", namespaces: []string{"ns1"}, createdAt: now},
				{id: "b", issuedBy: "u@x", name: "b", namespaces: []string{"ns1"}, createdAt: now},
				{id: "c", issuedBy: "u@x", name: "c", namespaces: []string{"ns1"}, createdAt: now},
				{id: "d", issuedBy: "u@x", name: "d", namespaces: []string{"ns1"}, createdAt: now},
				{id: "e", issuedBy: "u@x", name: "e", namespaces: []string{"ns1"}, createdAt: now},
			},
			filter:    domain.TokenFilter{AnyNamespace: true},
			params:    domain.TokenListParams{Sort: domain.SortParams{Field: "name"}, Offset: 2, Limit: 2},
			wantIDs:   []string{"c", "d"},
			wantTotal: 5,
		},
		{
			name: "sort by name asc",
			seeds: []seedToken{
				{id: "a", issuedBy: "u@x", name: "charlie", namespaces: []string{"ns1"}, createdAt: now},
				{id: "b", issuedBy: "u@x", name: "alpha", namespaces: []string{"ns1"}, createdAt: now},
				{id: "c", issuedBy: "u@x", name: "bravo", namespaces: []string{"ns1"}, createdAt: now},
			},
			filter:    domain.TokenFilter{AnyNamespace: true},
			params:    domain.TokenListParams{Sort: domain.SortParams{Field: "name"}},
			wantIDs:   []string{"b", "c", "a"},
			wantTotal: 3,
		},
		{
			name: "sort by last_used handles nil LastUsedAt",
			seeds: []seedToken{
				{
					id: "a", issuedBy: "u@x", name: "a",
					namespaces: []string{"ns1"}, createdAt: now, lastUsedAt: &earlier,
				},
				{
					id: "b", issuedBy: "u@x", name: "b",
					namespaces: []string{"ns1"}, createdAt: now, lastUsedAt: nil,
				},
				{
					id: "c", issuedBy: "u@x", name: "c",
					namespaces: []string{"ns1"}, createdAt: now, lastUsedAt: &now,
				},
			},
			filter:    domain.TokenFilter{AnyNamespace: true},
			params:    domain.TokenListParams{Sort: domain.SortParams{Field: "last_used"}},
			wantIDs:   []string{"b", "a", "c"},
			wantTotal: 3,
		},
		{
			name: "empty filter returns empty list",
			seeds: []seedToken{
				{id: "a", issuedBy: "u@x", name: "a", namespaces: []string{"ns1"}, createdAt: now},
				{id: "b", issuedBy: "u@x", name: "b", namespaces: []string{"ns2"}, createdAt: now},
			},
			filter:    domain.TokenFilter{AnyNamespace: false, NamespaceScopes: nil},
			wantIDs:   []string{},
			wantTotal: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := newTestStore(t)
			repo := bboltadapter.NewTokenRepo(store)
			ctx := t.Context()

			for _, s := range tt.seeds {
				require.NoError(t, repo.Create(ctx, mkSeed(s)))
			}

			got, total, err := repo.List(ctx, tt.filter, tt.params)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTotal, total)

			gotIDs := make([]string, 0, len(got))
			for _, tok := range got {
				gotIDs = append(gotIDs, tok.ID)
			}
			assert.Equal(t, tt.wantIDs, gotIDs)
		})
	}
}

func TestTokenRepo_Delete_ByID(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewTokenRepo(store)
	ctx := t.Context()

	token := newTestToken("token-del1", "henry@example.com", "hash-del1")
	require.NoError(t, repo.Create(ctx, token))

	require.NoError(t, repo.Delete(ctx, "token-del1"))

	_, err := repo.GetByHash(ctx, "hash-del1")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTokenRepo_Delete_Missing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewTokenRepo(store)
	ctx := t.Context()

	err := repo.Delete(ctx, "nonexistent-id")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTokenRepo_GetByID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T, repo *bboltadapter.TokenRepo)
		id      string
		wantErr error
		verify  func(t *testing.T, got *domain.Token)
	}{
		{
			name: "happy path returns correct Token",
			setup: func(t *testing.T, repo *bboltadapter.TokenRepo) {
				t.Helper()

				token := newTestToken("token-id-1", "getbyid@example.com", "hash-getbyid-1")
				require.NoError(t, repo.Create(t.Context(), token))
			},
			id: "token-id-1",
			verify: func(t *testing.T, got *domain.Token) {
				t.Helper()

				assert.Equal(t, "token-id-1", got.ID)
				assert.Equal(t, "getbyid@example.com", got.IssuedBy)
				assert.Equal(t, "hash-getbyid-1", got.TokenHash)
			},
		},
		{
			name:    "not found returns ErrNotFound",
			setup:   func(_ *testing.T, _ *bboltadapter.TokenRepo) {},
			id:      "nonexistent-id",
			wantErr: domain.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := newTestStore(t)
			repo := bboltadapter.NewTokenRepo(store)

			tt.setup(t, repo)

			got, err := repo.GetByID(t.Context(), tt.id)

			if tt.wantErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			tt.verify(t, got)
		})
	}
}

func TestTokenRepo_Delete_RemovesSecondaryIndex(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewTokenRepo(store)
	ctx := t.Context()

	token := newTestToken("token-sidx-del", "sidx@example.com", "hash-sidx-del")
	require.NoError(t, repo.Create(ctx, token))

	// Confirm both lookups work before deletion.
	_, err := repo.GetByID(ctx, "token-sidx-del")
	require.NoError(t, err)

	_, err = repo.GetByHash(ctx, "hash-sidx-del")
	require.NoError(t, err)

	// Delete via ID.
	require.NoError(t, repo.Delete(ctx, "token-sidx-del"))

	// Secondary index must be gone.
	_, err = repo.GetByID(ctx, "token-sidx-del")
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrNotFound)

	// Primary key must also be gone.
	_, err = repo.GetByHash(ctx, "hash-sidx-del")
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTokenRepo_UpdateLastUsed(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewTokenRepo(store)
	ctx := t.Context()

	token := newTestToken("token-upd1", "ivan@example.com", "hash-upd1")
	require.NoError(t, repo.Create(ctx, token))

	usedAt := time.Now().Add(time.Minute)
	require.NoError(t, repo.UpdateLastUsed(ctx, "hash-upd1", "192.168.1.1", usedAt))

	got, err := repo.GetByHash(ctx, "hash-upd1")
	require.NoError(t, err)
	require.NotNil(t, got.LastUsedAt)
	assert.Equal(t, usedAt.Unix(), got.LastUsedAt.Unix())
	assert.Equal(t, "192.168.1.1", got.LastUsedIP)
}
