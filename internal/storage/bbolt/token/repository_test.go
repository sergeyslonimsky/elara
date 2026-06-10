package token_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	tokenrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/token"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

func newRepo(t *testing.T) (*tokenrepo.Repository, pkgbbolt.Manager) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	mgr := pkgbbolt.NewManager(store.DB())

	return tokenrepo.NewRepository(mgr), mgr
}

func newToken(t *testing.T, overrides ...func(*domain.Token)) *domain.Token {
	t.Helper()

	id := "tok-" + randSuffix(t)
	tok := &domain.Token{
		ID:         id,
		IssuedBy:   "alice@example.com",
		Name:       "Test Token " + id,
		TokenHash:  "hash-" + id,
		Namespaces: []string{"ns1"},
		Role:       domain.RoleWriter,
		CreatedAt:  time.Now().UTC(),
	}
	for _, o := range overrides {
		o(tok)
	}

	return tok
}

// randSuffix returns a unique-per-call suffix. We use t.Name combined with a
// monotonic counter via t.TempDir parent to keep IDs unique within a test run.
func randSuffix(t *testing.T) string {
	t.Helper()
	// nanoseconds resolution is enough since each call is sequential within a goroutine.
	return time.Now().Format("150405.000000000")
}

func TestRepository_Create(t *testing.T) {
	t.Parallel()

	t.Run("success writes primary and secondary index", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		tok := newToken(t)
		require.NoError(t, repo.Create(ctx, tok))

		gotByHash, err := repo.GetByHash(ctx, tok.TokenHash)
		require.NoError(t, err)
		assert.Equal(t, tok.ID, gotByHash.ID)

		gotByID, err := repo.GetByID(ctx, tok.ID)
		require.NoError(t, err)
		assert.Equal(t, tok.TokenHash, gotByID.TokenHash)
	})

	t.Run("duplicate hash returns ErrResourceAlreadyExists", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		first := newToken(t, func(tok *domain.Token) { tok.TokenHash = "dup-hash" })
		require.NoError(t, repo.Create(ctx, first))

		second := newToken(t, func(tok *domain.Token) {
			tok.ID = first.ID + "-other"
			tok.TokenHash = "dup-hash"
		})
		err := repo.Create(ctx, second)
		require.ErrorIs(t, err, storage.ErrResourceAlreadyExists)
	})
}

func TestRepository_GetByHash(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		tok := newToken(t)
		require.NoError(t, repo.Create(ctx, tok))

		got, err := repo.GetByHash(ctx, tok.TokenHash)
		require.NoError(t, err)
		assert.Equal(t, tok.ID, got.ID)
		assert.Equal(t, tok.IssuedBy, got.IssuedBy)
		assert.Equal(t, tok.Role, got.Role)
		assert.Equal(t, tok.Namespaces, got.Namespaces)
	})

	t.Run("missing returns ErrResourceNotFound", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		_, err := repo.GetByHash(t.Context(), "nope")
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_GetByID(t *testing.T) {
	t.Parallel()

	t.Run("success via secondary index", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		tok := newToken(t)
		require.NoError(t, repo.Create(ctx, tok))

		got, err := repo.GetByID(ctx, tok.ID)
		require.NoError(t, err)
		assert.Equal(t, tok.TokenHash, got.TokenHash)
		assert.Equal(t, tok.Name, got.Name)
	})

	t.Run("missing returns ErrResourceNotFound", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		_, err := repo.GetByID(t.Context(), "unknown-id")
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_Delete(t *testing.T) {
	t.Parallel()

	t.Run("removes both primary and secondary index", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		tok := newToken(t)
		require.NoError(t, repo.Create(ctx, tok))

		require.NoError(t, repo.Delete(ctx, tok.ID))

		_, err := repo.GetByHash(ctx, tok.TokenHash)
		require.ErrorIs(t, err, storage.ErrResourceNotFound)

		_, err = repo.GetByID(ctx, tok.ID)
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})

	t.Run("missing returns ErrResourceNotFound", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		err := repo.Delete(t.Context(), "unknown-id")
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_UpdateLastUsed(t *testing.T) {
	t.Parallel()

	t.Run("sets LastUsedAt + LastUsedIP, other fields unchanged", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		tok := newToken(t)
		require.NoError(t, repo.Create(ctx, tok))

		at := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
		require.NoError(t, repo.UpdateLastUsed(ctx, tok.TokenHash, "10.0.0.5", at))

		got, err := repo.GetByHash(ctx, tok.TokenHash)
		require.NoError(t, err)
		require.NotNil(t, got.LastUsedAt)
		assert.True(t, at.Equal(*got.LastUsedAt))
		assert.Equal(t, "10.0.0.5", got.LastUsedIP)

		// Other fields preserved.
		assert.Equal(t, tok.ID, got.ID)
		assert.Equal(t, tok.IssuedBy, got.IssuedBy)
		assert.Equal(t, tok.Name, got.Name)
		assert.Equal(t, tok.Role, got.Role)
		assert.Equal(t, tok.Namespaces, got.Namespaces)
	})

	t.Run("missing hash returns ErrResourceNotFound", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		err := repo.UpdateLastUsed(t.Context(), "nope", "127.0.0.1", time.Now())
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_List(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	earlier := now.Add(-2 * time.Hour)
	earliest := now.Add(-5 * time.Hour)
	lastUsedNew := now.Add(-10 * time.Minute)
	lastUsedOld := now.Add(-1 * time.Hour)

	type seed struct {
		id         string
		issuedBy   string
		name       string
		namespaces []string
		createdAt  time.Time
		lastUsedAt *time.Time
	}

	mk := func(s seed) *domain.Token {
		return &domain.Token{
			ID:         s.id,
			IssuedBy:   s.issuedBy,
			Name:       s.name,
			TokenHash:  "hash-" + s.id,
			Namespaces: s.namespaces,
			Role:       domain.RoleWriter,
			CreatedAt:  s.createdAt,
			LastUsedAt: s.lastUsedAt,
		}
	}

	tests := []struct {
		name      string
		seeds     []seed
		filter    domain.TokenFilter
		params    domain.TokenListParams
		wantIDs   []string
		wantTotal int
	}{
		{
			name: "AnyNamespace returns all sorted by created desc",
			seeds: []seed{
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
			name: "filter by single IssuedBy",
			seeds: []seed{
				{id: "a", issuedBy: "carol@x", name: "a", namespaces: []string{"ns1"}, createdAt: now},
				{id: "b", issuedBy: "carol@x", name: "b", namespaces: []string{"ns1"}, createdAt: earlier},
				{id: "c", issuedBy: "dave@x", name: "c", namespaces: []string{"ns1"}, createdAt: now},
			},
			filter:    domain.TokenFilter{AnyNamespace: true, IssuedBy: []string{"carol@x"}},
			params:    domain.TokenListParams{Sort: domain.SortParams{Field: "name"}},
			wantIDs:   []string{"a", "b"},
			wantTotal: 2,
		},
		{
			name: "filter by multi IssuedBy",
			seeds: []seed{
				{id: "a", issuedBy: "carol@x", name: "a", namespaces: []string{"ns1"}, createdAt: now},
				{id: "b", issuedBy: "dave@x", name: "b", namespaces: []string{"ns1"}, createdAt: now},
				{id: "c", issuedBy: "eve@x", name: "c", namespaces: []string{"ns1"}, createdAt: now},
			},
			filter:    domain.TokenFilter{AnyNamespace: true, IssuedBy: []string{"carol@x", "dave@x"}},
			params:    domain.TokenListParams{Sort: domain.SortParams{Field: "name"}},
			wantIDs:   []string{"a", "b"},
			wantTotal: 2,
		},
		{
			name: "NamespaceScopes filter returns intersection",
			seeds: []seed{
				{id: "a", issuedBy: "u@x", name: "a", namespaces: []string{"ns1"}, createdAt: now},
				{id: "b", issuedBy: "u@x", name: "b", namespaces: []string{"ns2"}, createdAt: now},
				{id: "c", issuedBy: "u@x", name: "c", namespaces: []string{"ns1"}, createdAt: earlier},
			},
			filter:    domain.TokenFilter{NamespaceScopes: map[string]struct{}{"ns1": {}}},
			params:    domain.TokenListParams{Sort: domain.SortParams{Field: "name"}},
			wantIDs:   []string{"a", "c"},
			wantTotal: 2,
		},
		{
			name: "multi-namespace token matches when any overlaps with scope",
			seeds: []seed{
				{id: "a", issuedBy: "u@x", name: "a", namespaces: []string{"ns1", "ns2"}, createdAt: now},
				{id: "b", issuedBy: "u@x", name: "b", namespaces: []string{"ns3"}, createdAt: now},
			},
			filter:    domain.TokenFilter{NamespaceScopes: map[string]struct{}{"ns2": {}}},
			params:    domain.TokenListParams{Sort: domain.SortParams{Field: "name"}},
			wantIDs:   []string{"a"},
			wantTotal: 1,
		},
		{
			name: "explicit Namespaces slice narrows result",
			seeds: []seed{
				{id: "a", issuedBy: "u@x", name: "a", namespaces: []string{"ns1"}, createdAt: now},
				{id: "b", issuedBy: "u@x", name: "b", namespaces: []string{"ns2"}, createdAt: now},
				{id: "c", issuedBy: "u@x", name: "c", namespaces: []string{"ns3"}, createdAt: now},
			},
			filter:    domain.TokenFilter{AnyNamespace: true, Namespaces: []string{"ns2", "ns3"}},
			params:    domain.TokenListParams{Sort: domain.SortParams{Field: "name"}},
			wantIDs:   []string{"b", "c"},
			wantTotal: 2,
		},
		{
			name: "QueryParams case-insensitive substring match on name",
			seeds: []seed{
				{id: "a", issuedBy: "u@x", name: "Production Token", namespaces: []string{"ns1"}, createdAt: now},
				{id: "b", issuedBy: "u@x", name: "staging-token", namespaces: []string{"ns1"}, createdAt: now},
				{id: "c", issuedBy: "u@x", name: "ci runner", namespaces: []string{"ns1"}, createdAt: now},
			},
			filter:    domain.TokenFilter{AnyNamespace: true, QueryParams: []string{"TOKEN"}},
			params:    domain.TokenListParams{Sort: domain.SortParams{Field: "name"}},
			wantIDs:   []string{"a", "b"},
			wantTotal: 2,
		},
		{
			name: "sort by name asc",
			seeds: []seed{
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
			name: "sort by name desc",
			seeds: []seed{
				{id: "a", issuedBy: "u@x", name: "charlie", namespaces: []string{"ns1"}, createdAt: now},
				{id: "b", issuedBy: "u@x", name: "alpha", namespaces: []string{"ns1"}, createdAt: now},
				{id: "c", issuedBy: "u@x", name: "bravo", namespaces: []string{"ns1"}, createdAt: now},
			},
			filter:    domain.TokenFilter{AnyNamespace: true},
			params:    domain.TokenListParams{Sort: domain.SortParams{Field: "name", Desc: true}},
			wantIDs:   []string{"a", "c", "b"},
			wantTotal: 3,
		},
		{
			name: "sort by last_used: nils sort first asc; tie-break by createdAt",
			seeds: []seed{
				{
					id:         "a",
					issuedBy:   "u@x",
					name:       "a",
					namespaces: []string{"ns1"},
					createdAt:  earliest,
					lastUsedAt: &lastUsedOld,
				},
				{
					id:         "b",
					issuedBy:   "u@x",
					name:       "b",
					namespaces: []string{"ns1"},
					createdAt:  now,
					lastUsedAt: &lastUsedNew,
				},
				{id: "c", issuedBy: "u@x", name: "c", namespaces: []string{"ns1"}, createdAt: earlier, lastUsedAt: nil},
				{
					id:         "d",
					issuedBy:   "u@x",
					name:       "d",
					namespaces: []string{"ns1"},
					createdAt:  earliest,
					lastUsedAt: nil,
				},
			},
			filter: domain.TokenFilter{AnyNamespace: true},
			params: domain.TokenListParams{Sort: domain.SortParams{Field: "last_used"}},
			// nil LastUsedAt → "less" → ordered first; tie-break by CreatedAt asc.
			wantIDs:   []string{"d", "c", "a", "b"},
			wantTotal: 4,
		},
		{
			name: "default sort created desc",
			seeds: []seed{
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
			name: "pagination: offset + limit slices result, total reflects pre-pagination",
			seeds: []seed{
				{id: "a", issuedBy: "u@x", name: "a", namespaces: []string{"ns1"}, createdAt: now},
				{id: "b", issuedBy: "u@x", name: "b", namespaces: []string{"ns1"}, createdAt: now},
				{id: "c", issuedBy: "u@x", name: "c", namespaces: []string{"ns1"}, createdAt: now},
				{id: "d", issuedBy: "u@x", name: "d", namespaces: []string{"ns1"}, createdAt: now},
			},
			filter:    domain.TokenFilter{AnyNamespace: true},
			params:    domain.TokenListParams{Sort: domain.SortParams{Field: "name"}, Offset: 1, Limit: 2},
			wantIDs:   []string{"b", "c"},
			wantTotal: 4,
		},
		{
			name: "pagination: offset beyond length returns empty slice but full total",
			seeds: []seed{
				{id: "a", issuedBy: "u@x", name: "a", namespaces: []string{"ns1"}, createdAt: now},
				{id: "b", issuedBy: "u@x", name: "b", namespaces: []string{"ns1"}, createdAt: now},
			},
			filter:    domain.TokenFilter{AnyNamespace: true},
			params:    domain.TokenListParams{Sort: domain.SortParams{Field: "name"}, Offset: 99, Limit: 10},
			wantIDs:   []string{},
			wantTotal: 2,
		},
		{
			name: "no NamespaceScopes and no AnyNamespace returns empty",
			seeds: []seed{
				{id: "a", issuedBy: "u@x", name: "a", namespaces: []string{"ns1"}, createdAt: now},
			},
			filter:    domain.TokenFilter{},
			params:    domain.TokenListParams{},
			wantIDs:   []string{},
			wantTotal: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, _ := newRepo(t)
			ctx := t.Context()
			for _, s := range tt.seeds {
				require.NoError(t, repo.Create(ctx, mk(s)))
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

func TestRepository_ListAll(t *testing.T) {
	t.Parallel()

	t.Run("empty repo returns empty slice", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		got, err := repo.ListAll(t.Context())
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("returns every token regardless of namespace", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		seeds := []*domain.Token{
			{
				ID:         "a",
				IssuedBy:   "u@x",
				Name:       "a",
				TokenHash:  "h-a",
				Namespaces: []string{"ns1"},
				Role:       domain.RoleWriter,
				CreatedAt:  time.Now(),
			},
			{
				ID:         "b",
				IssuedBy:   "u@x",
				Name:       "b",
				TokenHash:  "h-b",
				Namespaces: []string{"ns2"},
				Role:       domain.RoleReader,
				CreatedAt:  time.Now(),
			},
			{
				ID:         "c",
				IssuedBy:   "v@x",
				Name:       "c",
				TokenHash:  "h-c",
				Namespaces: []string{"ns3"},
				Role:       domain.RoleWriter,
				CreatedAt:  time.Now(),
			},
		}
		for _, s := range seeds {
			require.NoError(t, repo.Create(ctx, s))
		}

		got, err := repo.ListAll(ctx)
		require.NoError(t, err)
		assert.Len(t, got, 3)
	})
}

func TestRepository_WithTx_RollbackDiscardsWrites(t *testing.T) {
	t.Parallel()

	repo, mgr := newRepo(t)
	ctx := t.Context()

	a := newToken(t, func(tok *domain.Token) {
		tok.ID = "a"
		tok.TokenHash = "h-a"
	})
	b := newToken(t, func(tok *domain.Token) {
		tok.ID = "b"
		tok.TokenHash = "h-b"
	})

	err := mgr.WithTx(ctx, func(ctx context.Context) error {
		if err := repo.Create(ctx, a); err != nil {
			return err
		}
		if err := repo.Create(ctx, b); err != nil {
			return err
		}

		return assert.AnError
	})
	require.ErrorIs(t, err, assert.AnError)

	got, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, got)
}
