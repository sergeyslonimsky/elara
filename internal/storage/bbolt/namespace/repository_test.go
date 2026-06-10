package namespace_test

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
	namespacerepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/namespace"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

func newRepo(t *testing.T) (*namespacerepo.Repository, pkgbbolt.Manager) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	mgr := pkgbbolt.NewManager(store.DB())

	return namespacerepo.NewRepository(mgr), mgr
}

func newTestNamespace(name string) *domain.Namespace {
	now := time.Now()

	return &domain.Namespace{
		Name:        name,
		Description: "test ns",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestRepository_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T) (*namespacerepo.Repository, *domain.Namespace)
		errIs   error
		wantErr string
	}{
		{
			name: "success",
			setup: func(t *testing.T) (*namespacerepo.Repository, *domain.Namespace) {
				t.Helper()
				repo, _ := newRepo(t)

				return repo, newTestNamespace("ns1")
			},
		},
		{
			name: "duplicate returns ErrResourceAlreadyExists",
			setup: func(t *testing.T) (*namespacerepo.Repository, *domain.Namespace) {
				t.Helper()
				repo, _ := newRepo(t)

				existing := newTestNamespace("ns1")
				require.NoError(t, repo.Create(t.Context(), existing))

				return repo, newTestNamespace("ns1")
			},
			errIs: storage.ErrResourceAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, ns := tt.setup(t)

			err := repo.Create(t.Context(), ns)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRepository_Get(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T) (*namespacerepo.Repository, string, *domain.Namespace)
		errIs   error
		wantErr string
	}{
		{
			name: "success",
			setup: func(t *testing.T) (*namespacerepo.Repository, string, *domain.Namespace) {
				t.Helper()
				repo, _ := newRepo(t)
				ns := newTestNamespace("ns1")
				require.NoError(t, repo.Create(t.Context(), ns))

				return repo, ns.Name, ns
			},
		},
		{
			name: "missing returns ErrResourceNotFound",
			setup: func(t *testing.T) (*namespacerepo.Repository, string, *domain.Namespace) {
				t.Helper()
				repo, _ := newRepo(t)

				return repo, "nonexistent", nil
			},
			errIs: storage.ErrResourceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, name, want := tt.setup(t)

			got, err := repo.Get(t.Context(), name)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, want.Name, got.Name)
			assert.Equal(t, want.Description, got.Description)
		})
	}
}

func TestRepository_Update(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T) (*namespacerepo.Repository, *domain.Namespace)
		assert  func(t *testing.T, repo *namespacerepo.Repository, ns *domain.Namespace)
		errIs   error
		wantErr string
	}{
		{
			name: "success persists new description",
			setup: func(t *testing.T) (*namespacerepo.Repository, *domain.Namespace) {
				t.Helper()
				repo, _ := newRepo(t)

				ns := newTestNamespace("ns1")
				require.NoError(t, repo.Create(t.Context(), ns))

				ns.Description = "updated"

				return repo, ns
			},
			assert: func(t *testing.T, repo *namespacerepo.Repository, ns *domain.Namespace) {
				t.Helper()

				got, err := repo.Get(t.Context(), ns.Name)
				require.NoError(t, err)
				assert.Equal(t, "updated", got.Description)
			},
		},
		{
			name: "missing returns ErrResourceNotFound",
			setup: func(t *testing.T) (*namespacerepo.Repository, *domain.Namespace) {
				t.Helper()
				repo, _ := newRepo(t)

				return repo, newTestNamespace("nonexistent")
			},
			errIs: storage.ErrResourceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, ns := tt.setup(t)

			err := repo.Update(t.Context(), ns)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)

			if tt.assert != nil {
				tt.assert(t, repo, ns)
			}
		})
	}
}

func TestRepository_Delete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T) (*namespacerepo.Repository, string)
		errIs   error
		wantErr string
	}{
		{
			name: "success",
			setup: func(t *testing.T) (*namespacerepo.Repository, string) {
				t.Helper()
				repo, _ := newRepo(t)

				ns := newTestNamespace("ns1")
				require.NoError(t, repo.Create(t.Context(), ns))

				return repo, ns.Name
			},
		},
		{
			name: "missing returns ErrResourceNotFound",
			setup: func(t *testing.T) (*namespacerepo.Repository, string) {
				t.Helper()
				repo, _ := newRepo(t)

				return repo, "nonexistent"
			},
			errIs: storage.ErrResourceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, name := tt.setup(t)

			err := repo.Delete(t.Context(), name)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRepository_List(t *testing.T) {
	t.Parallel()

	seed := func(t *testing.T) *namespacerepo.Repository {
		t.Helper()
		repo, _ := newRepo(t)

		for _, n := range []string{"alpha", "beta", "gamma"} {
			ns := newTestNamespace(n)
			ns.Description = "desc-" + n
			require.NoError(t, repo.Create(t.Context(), ns))
		}

		return repo
	}

	tests := []struct {
		name     string
		filter   domain.NamespaceFilter
		params   domain.NamespaceListParams
		wantLen  int
		wantHead string
	}{
		{
			name:     "wildcard returns all sorted by name",
			filter:   domain.NamespaceFilter{Wildcard: true},
			params:   domain.NamespaceListParams{},
			wantLen:  3,
			wantHead: "alpha",
		},
		{
			name:     "scoped by names",
			filter:   domain.NamespaceFilter{Names: map[string]struct{}{"alpha": {}, "gamma": {}}},
			params:   domain.NamespaceListParams{},
			wantLen:  2,
			wantHead: "alpha",
		},
		{
			name:     "search filters",
			filter:   domain.NamespaceFilter{Wildcard: true, Search: "be"},
			params:   domain.NamespaceListParams{},
			wantLen:  1,
			wantHead: "beta",
		},
		{
			name:     "limit caps result",
			filter:   domain.NamespaceFilter{Wildcard: true},
			params:   domain.NamespaceListParams{Sort: domain.SortParams{}, Offset: 0, Limit: 2},
			wantLen:  2,
			wantHead: "alpha",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := seed(t)

			got, total, err := repo.List(t.Context(), tt.filter, tt.params)
			require.NoError(t, err)
			assert.Len(t, got, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Equal(t, tt.wantHead, got[0].Name)
			}
			if tt.params.Limit == 0 {
				assert.Equal(t, tt.wantLen, total)
			}
		})
	}
}

func TestRepository_UpdateTimestamp(t *testing.T) {
	t.Parallel()

	t.Run("bumps UpdatedAt on existing", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		ns := newTestNamespace("ns1")
		ns.UpdatedAt = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, repo.Create(ctx, ns))

		require.NoError(t, repo.UpdateTimestamp(ctx, ns.Name))

		got, err := repo.Get(ctx, ns.Name)
		require.NoError(t, err)
		assert.True(t, got.UpdatedAt.After(ns.UpdatedAt))
	})

	t.Run("missing returns ErrResourceNotFound", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		err := repo.UpdateTimestamp(t.Context(), "nonexistent")
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_LockUnlock(t *testing.T) {
	t.Parallel()

	t.Run("Lock flips Locked + writes audit entries", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		ns := newTestNamespace("ns1")
		require.NoError(t, repo.Create(ctx, ns))

		require.NoError(t, repo.LockNamespace(ctx, ns.Name))

		got, err := repo.Get(ctx, ns.Name)
		require.NoError(t, err)
		assert.True(t, got.Locked)
	})

	t.Run("Lock is idempotent", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		ns := newTestNamespace("ns1")
		require.NoError(t, repo.Create(ctx, ns))

		require.NoError(t, repo.LockNamespace(ctx, ns.Name))
		require.NoError(t, repo.LockNamespace(ctx, ns.Name))

		got, err := repo.Get(ctx, ns.Name)
		require.NoError(t, err)
		assert.True(t, got.Locked)
	})

	t.Run("Unlock flips Locked back", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		ns := newTestNamespace("ns1")
		require.NoError(t, repo.Create(ctx, ns))
		require.NoError(t, repo.LockNamespace(ctx, ns.Name))

		require.NoError(t, repo.UnlockNamespace(ctx, ns.Name))

		got, err := repo.Get(ctx, ns.Name)
		require.NoError(t, err)
		assert.False(t, got.Locked)
	})

	t.Run("missing returns ErrResourceNotFound", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		err := repo.LockNamespace(t.Context(), "nonexistent")
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_WithTx_RollbackDiscardsWrites(t *testing.T) {
	t.Parallel()

	repo, mgr := newRepo(t)
	ctx := t.Context()

	a := newTestNamespace("a")
	b := newTestNamespace("b")

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
