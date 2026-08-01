package config_test

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
	configrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/config"
	namespacerepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/namespace"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

func newRepo(t *testing.T) (*configrepo.Repository, *namespacerepo.Repository, pkgbbolt.Manager) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	mgr := pkgbbolt.NewManager(store.DB())

	return configrepo.NewRepository(mgr), namespacerepo.NewRepository(mgr), mgr
}

func seedNamespace(t *testing.T, ns *namespacerepo.Repository, name string) {
	t.Helper()

	require.NoError(t, ns.Create(t.Context(), &domain.Namespace{
		Name:        name,
		Description: "test ns",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}))
}

func newTestConfig(namespace, path, content string) *domain.Config {
	return &domain.Config{
		Path:      path,
		Namespace: namespace,
		Content:   content,
		Format:    domain.FormatJSON,
		Metadata:  map[string]string{},
	}
}

func TestRepository_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T) (*configrepo.Repository, *domain.Config)
		errIs   error
		wantErr string
	}{
		{
			name: "success persists fields and assigns revision 1",
			setup: func(t *testing.T) (*configrepo.Repository, *domain.Config) {
				t.Helper()
				repo, nsr, _ := newRepo(t)
				seedNamespace(t, nsr, "ns")

				return repo, newTestConfig("ns", "/a", `{"k":1}`)
			},
		},
		{
			name: "duplicate returns ErrResourceAlreadyExists",
			setup: func(t *testing.T) (*configrepo.Repository, *domain.Config) {
				t.Helper()
				repo, nsr, _ := newRepo(t)
				seedNamespace(t, nsr, "ns")

				require.NoError(t, repo.Create(t.Context(), newTestConfig("ns", "/a", "v1")))

				return repo, newTestConfig("ns", "/a", "v2")
			},
			errIs: storage.ErrResourceAlreadyExists,
		},
		{
			name: "locked namespace returns ErrNamespaceLocked",
			setup: func(t *testing.T) (*configrepo.Repository, *domain.Config) {
				t.Helper()
				repo, nsr, _ := newRepo(t)
				seedNamespace(t, nsr, "ns")
				require.NoError(t, nsr.LockNamespace(t.Context(), "ns"))

				return repo, newTestConfig("ns", "/a", "v1")
			},
			errIs: domain.ErrNamespaceLocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, cfg := tt.setup(t)

			err := repo.Create(t.Context(), cfg)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)

			got, err := repo.Get(t.Context(), cfg.Path, cfg.Namespace)
			require.NoError(t, err)
			assert.Equal(t, cfg.Content, got.Content)
			assert.NotEmpty(t, got.ContentHash)
			assert.Equal(t, int64(1), got.Version)
			assert.Equal(t, int64(1), got.Revision)
			assert.Equal(t, int64(1), got.CreateRevision)
			assert.False(t, got.CreatedAt.IsZero())
			assert.False(t, got.UpdatedAt.IsZero())
		})
	}
}

func TestRepository_Get_NotFound(t *testing.T) {
	t.Parallel()

	repo, _, _ := newRepo(t)

	_, err := repo.Get(t.Context(), "/missing", "ns")
	require.ErrorIs(t, err, storage.ErrResourceNotFound)
}

func TestRepository_Update(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T) (*configrepo.Repository, *domain.Config)
		errIs   error
		wantErr string
	}{
		{
			name: "success bumps revision and version",
			setup: func(t *testing.T) (*configrepo.Repository, *domain.Config) {
				t.Helper()
				repo, nsr, _ := newRepo(t)
				seedNamespace(t, nsr, "ns")
				orig := newTestConfig("ns", "/a", "v1")
				require.NoError(t, repo.Create(t.Context(), orig))

				orig.Content = "v2"

				return repo, orig
			},
		},
		{
			name: "stale version returns version conflict",
			setup: func(t *testing.T) (*configrepo.Repository, *domain.Config) {
				t.Helper()
				repo, nsr, _ := newRepo(t)
				seedNamespace(t, nsr, "ns")
				orig := newTestConfig("ns", "/a", "v1")
				require.NoError(t, repo.Create(t.Context(), orig))

				stale := newTestConfig("ns", "/a", "v2")
				stale.Version = 999

				return repo, stale
			},
			errIs: domain.ErrVersionConflict,
		},
		{
			name: "missing returns ErrResourceNotFound",
			setup: func(t *testing.T) (*configrepo.Repository, *domain.Config) {
				t.Helper()
				repo, nsr, _ := newRepo(t)
				seedNamespace(t, nsr, "ns")

				cfg := newTestConfig("ns", "/missing", "v1")
				cfg.Version = 1

				return repo, cfg
			},
			errIs: storage.ErrResourceNotFound,
		},
		{
			name: "locked namespace returns ErrNamespaceLocked",
			setup: func(t *testing.T) (*configrepo.Repository, *domain.Config) {
				t.Helper()
				repo, nsr, _ := newRepo(t)
				seedNamespace(t, nsr, "ns")
				orig := newTestConfig("ns", "/a", "v1")
				require.NoError(t, repo.Create(t.Context(), orig))

				require.NoError(t, nsr.LockNamespace(t.Context(), "ns"))

				orig.Content = "v2"

				return repo, orig
			},
			errIs: domain.ErrNamespaceLocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, cfg := tt.setup(t)
			origCreateRev := cfg.CreateRevision

			err := repo.Update(t.Context(), cfg)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)

			got, err := repo.Get(t.Context(), cfg.Path, cfg.Namespace)
			require.NoError(t, err)
			assert.Equal(t, "v2", got.Content)
			assert.Equal(t, int64(2), got.Version)
			assert.Equal(t, int64(2), got.Revision)
			assert.Equal(t, origCreateRev, got.CreateRevision)
		})
	}
}

func TestRepository_Delete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T) (*configrepo.Repository, string, string)
		errIs   error
		wantErr string
	}{
		{
			name: "success returns new revision",
			setup: func(t *testing.T) (*configrepo.Repository, string, string) {
				t.Helper()
				repo, nsr, _ := newRepo(t)
				seedNamespace(t, nsr, "ns")
				require.NoError(t, repo.Create(t.Context(), newTestConfig("ns", "/a", "v1")))

				return repo, "/a", "ns"
			},
		},
		{
			name: "missing returns ErrResourceNotFound",
			setup: func(t *testing.T) (*configrepo.Repository, string, string) {
				t.Helper()
				repo, nsr, _ := newRepo(t)
				seedNamespace(t, nsr, "ns")

				return repo, "/missing", "ns"
			},
			errIs: storage.ErrResourceNotFound,
		},
		{
			name: "locked config returns ErrLocked",
			setup: func(t *testing.T) (*configrepo.Repository, string, string) {
				t.Helper()
				repo, nsr, _ := newRepo(t)
				seedNamespace(t, nsr, "ns")
				require.NoError(t, repo.Create(t.Context(), newTestConfig("ns", "/a", "v1")))
				require.NoError(t, repo.LockConfig(t.Context(), "ns", "/a"))

				return repo, "/a", "ns"
			},
			errIs: domain.ErrLocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, path, ns := tt.setup(t)

			rev, err := repo.Delete(t.Context(), path, ns)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Positive(t, rev)

			_, err = repo.Get(t.Context(), path, ns)
			require.ErrorIs(t, err, storage.ErrResourceNotFound)

			changes, err := repo.ListChanges(t.Context(), 0, 100)
			require.NoError(t, err)
			found := false
			for _, c := range changes {
				if c.Type == domain.EventTypeDeleted && c.Path == path {
					found = true
				}
			}
			assert.True(t, found, "expected deletion changelog entry")
		})
	}
}

func TestRepository_LockUnlock(t *testing.T) {
	t.Parallel()

	t.Run("lock flips Locked + blocks update + blocks delete + appears in history", func(t *testing.T) {
		t.Parallel()
		repo, nsr, _ := newRepo(t)
		ctx := t.Context()
		seedNamespace(t, nsr, "ns")
		require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/a", "v1")))

		require.NoError(t, repo.LockConfig(ctx, "ns", "/a"))

		got, err := repo.Get(ctx, "/a", "ns")
		require.NoError(t, err)
		assert.True(t, got.Locked)

		got.Content = "v2"
		err = repo.Update(ctx, got)
		require.ErrorIs(t, err, domain.ErrLocked)

		_, err = repo.Delete(ctx, "/a", "ns")
		require.ErrorIs(t, err, domain.ErrLocked)

		hist, err := repo.GetConfigHistory(ctx, "/a", "ns", 100)
		require.NoError(t, err)
		hasLock := false
		for _, h := range hist {
			if h.EventType == domain.EventTypeLocked {
				hasLock = true
			}
		}
		assert.True(t, hasLock, "expected lock event in history")
	})

	t.Run("unlock flips Locked back", func(t *testing.T) {
		t.Parallel()
		repo, nsr, _ := newRepo(t)
		ctx := t.Context()
		seedNamespace(t, nsr, "ns")
		require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/a", "v1")))
		require.NoError(t, repo.LockConfig(ctx, "ns", "/a"))
		require.NoError(t, repo.UnlockConfig(ctx, "ns", "/a"))

		got, err := repo.Get(ctx, "/a", "ns")
		require.NoError(t, err)
		assert.False(t, got.Locked)
	})

	t.Run("lock missing returns ErrResourceNotFound", func(t *testing.T) {
		t.Parallel()
		repo, nsr, _ := newRepo(t)
		seedNamespace(t, nsr, "ns")

		err := repo.LockConfig(t.Context(), "ns", "/missing")
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})

	t.Run("locking an already-locked config is a no-op", func(t *testing.T) {
		t.Parallel()
		repo, nsr, _ := newRepo(t)
		ctx := t.Context()
		seedNamespace(t, nsr, "ns")
		require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/a", "v1")))

		require.NoError(t, repo.LockConfig(ctx, "ns", "/a"))
		require.NoError(t, repo.LockConfig(ctx, "ns", "/a"))

		got, err := repo.Get(ctx, "/a", "ns")
		require.NoError(t, err)
		assert.True(t, got.Locked)
	})

	t.Run("unlocking an already-unlocked config is a no-op", func(t *testing.T) {
		t.Parallel()
		repo, nsr, _ := newRepo(t)
		ctx := t.Context()
		seedNamespace(t, nsr, "ns")
		require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/a", "v1")))

		require.NoError(t, repo.UnlockConfig(ctx, "ns", "/a"))

		got, err := repo.Get(ctx, "/a", "ns")
		require.NoError(t, err)
		assert.False(t, got.Locked)
	})
}

func TestRepository_CurrentRevision(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	rev, err := repo.CurrentRevision(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rev)

	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/a", "v1")))
	rev, err = repo.CurrentRevision(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rev)

	cfg, err := repo.Get(ctx, "/a", "ns")
	require.NoError(t, err)
	cfg.Content = "v2"
	require.NoError(t, repo.Update(ctx, cfg))

	rev, err = repo.CurrentRevision(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), rev)

	_, err = repo.Delete(ctx, "/a", "ns")
	require.NoError(t, err)

	rev, err = repo.CurrentRevision(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), rev)
}

func TestRepository_GetAtRevision(t *testing.T) {
	t.Parallel()

	t.Run("returns historical content", func(t *testing.T) {
		t.Parallel()
		repo, nsr, _ := newRepo(t)
		ctx := t.Context()
		seedNamespace(t, nsr, "ns")

		require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/a", "v1")))
		cfg, err := repo.Get(ctx, "/a", "ns")
		require.NoError(t, err)
		cfg.Content = "v2"
		require.NoError(t, repo.Update(ctx, cfg))

		entry, err := repo.GetAtRevision(ctx, "/a", "ns", 1)
		require.NoError(t, err)
		assert.Equal(t, int64(1), entry.Revision)
		assert.Equal(t, "v1", entry.Content)

		entry, err = repo.GetAtRevision(ctx, "/a", "ns", 2)
		require.NoError(t, err)
		assert.Equal(t, "v2", entry.Content)
	})

	t.Run("missing returns ErrResourceNotFound", func(t *testing.T) {
		t.Parallel()
		repo, nsr, _ := newRepo(t)
		seedNamespace(t, nsr, "ns")

		_, err := repo.GetAtRevision(t.Context(), "/missing", "ns", 1)
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_ListChanges(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")
	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/a", "v1")))
	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/b", "v1")))

	all, err := repo.ListChanges(ctx, 0, 100)
	require.NoError(t, err)
	assert.Len(t, all, 2)

	tail, err := repo.ListChanges(ctx, 1, 100)
	require.NoError(t, err)
	require.Len(t, tail, 1)
	assert.Equal(t, "/b", tail[0].Path)

	limited, err := repo.ListChanges(ctx, 0, 1)
	require.NoError(t, err)
	assert.Len(t, limited, 1)
}

func TestRepository_ListRecentChanges(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")
	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/a", "v1")))
	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/b", "v1")))
	require.NoError(t, repo.LockConfig(ctx, "ns", "/a"))

	out, err := repo.ListRecentChanges(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, out, 2)
}

func TestRepository_ListByPrefix(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")
	seedNamespace(t, nsr, "other")

	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/svc/a", "v")))
	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/svc/b", "v")))
	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/other/c", "v")))
	require.NoError(t, repo.Create(ctx, newTestConfig("other", "/svc/d", "v")))

	got, err := repo.ListByPrefix(ctx, "/svc", "ns")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "/svc/a", got[0].Path)
	assert.Equal(t, "/svc/b", got[1].Path)
}

func TestRepository_ListByPrefix_CrossNamespaceFiltersByPath(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns1")
	seedNamespace(t, nsr, "ns2")

	require.NoError(t, repo.Create(ctx, newTestConfig("ns1", "/match/a", "v")))
	require.NoError(t, repo.Create(ctx, newTestConfig("ns2", "/match/b", "v")))
	require.NoError(t, repo.Create(ctx, newTestConfig("ns1", "/other/c", "v")))

	// Empty namespace triggers a full scan; shouldSkipByPath filters entries
	// whose path does not carry the requested prefix.
	got, err := repo.ListByPrefix(ctx, "/match", "")
	require.NoError(t, err)
	require.Len(t, got, 2)
	for _, c := range got {
		assert.Contains(t, c.Path, "/match")
	}
}

func TestRepository_ListAllByNamespace(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")
	seedNamespace(t, nsr, "other")

	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/svc/a", "v")))
	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/other/c", "v")))
	require.NoError(t, repo.Create(ctx, newTestConfig("other", "/svc/d", "v")))

	got, err := repo.ListAllByNamespace(ctx, "ns")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "/other/c", got[0].Path)
	assert.Equal(t, "/svc/a", got[1].Path)
}

func TestRepository_ListSummariesByPrefix(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/svc/a", "v")))
	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/svc/b", "v")))
	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/other/c", "v")))

	got, err := repo.ListSummariesByPrefix(ctx, "/svc", "ns")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "/svc/a", got[0].Path)
	assert.Equal(t, "/svc/b", got[1].Path)
	assert.False(t, got[0].NamespaceLocked)
}

func TestRepository_ListConfigPage(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	for _, p := range []string{"/a", "/b", "/c", "/d"} {
		require.NoError(t, repo.Create(ctx, newTestConfig("ns", p, "v")))
	}

	page, total, err := repo.ListConfigPage(ctx, "", "ns", 2, 1)
	require.NoError(t, err)
	assert.Equal(t, 4, total)
	require.Len(t, page, 2)
	assert.Equal(t, "/b", page[0].Path)
	assert.Equal(t, "v", page[0].Content)
	assert.Equal(t, "/c", page[1].Path)
}

func TestRepository_CountByNamespace(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	n, err := repo.CountByNamespace(ctx, "ns")
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/a", "v")))
	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/b", "v")))

	n, err = repo.CountByNamespace(ctx, "ns")
	require.NoError(t, err)
	assert.Equal(t, 2, n)
}

func TestRepository_SearchByPath(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/Service/Alpha", "v")))
	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/Service/Beta", "v")))
	require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/other/x", "v")))

	got, err := repo.SearchByPath(ctx, "SERVICE", "ns")
	require.NoError(t, err)
	assert.Len(t, got, 2)

	got, err = repo.SearchByPath(ctx, "alpha", "ns")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "/Service/Alpha", got[0].Path)
}

func TestRepository_ListSummaryPage(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	for _, p := range []string{"/a", "/b", "/c", "/d"} {
		require.NoError(t, repo.Create(ctx, newTestConfig("ns", p, "v")))
	}

	page, total, err := repo.ListSummaryPage(ctx, "", "ns", 2, 1)
	require.NoError(t, err)
	assert.Equal(t, 4, total)
	require.Len(t, page, 2)
	assert.Equal(t, "/b", page[0].Path)
	assert.Equal(t, "/c", page[1].Path)
}

func TestRepository_GetConfigHistory_Merges(t *testing.T) {
	t.Parallel()

	t.Run("create + lock + update produce 3 entries", func(t *testing.T) {
		t.Parallel()
		repo, nsr, _ := newRepo(t)
		ctx := t.Context()
		seedNamespace(t, nsr, "ns")
		require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/a", "v1")))
		require.NoError(t, repo.LockConfig(ctx, "ns", "/a"))
		require.NoError(t, repo.UnlockConfig(ctx, "ns", "/a"))

		cfg, err := repo.Get(ctx, "/a", "ns")
		require.NoError(t, err)
		cfg.Content = "v2"
		require.NoError(t, repo.Update(ctx, cfg))

		hist, err := repo.GetConfigHistory(ctx, "/a", "ns", 100)
		require.NoError(t, err)
		assert.Len(t, hist, 4)

		seen := map[domain.EventType]bool{}
		for _, h := range hist {
			seen[h.EventType] = true
		}
		assert.True(t, seen[domain.EventTypeCreated])
		assert.True(t, seen[domain.EventTypeLocked])
		assert.True(t, seen[domain.EventTypeUnlocked])
		assert.True(t, seen[domain.EventTypeUpdated])
	})

	t.Run("namespace lock event surfaces in config history", func(t *testing.T) {
		t.Parallel()
		repo, nsr, _ := newRepo(t)
		ctx := t.Context()
		seedNamespace(t, nsr, "ns")
		require.NoError(t, repo.Create(ctx, newTestConfig("ns", "/a", "v1")))
		require.NoError(t, nsr.LockNamespace(ctx, "ns"))
		require.NoError(t, nsr.UnlockNamespace(ctx, "ns"))

		hist, err := repo.GetConfigHistory(ctx, "/a", "ns", 100)
		require.NoError(t, err)

		seen := map[domain.EventType]bool{}
		for _, h := range hist {
			seen[h.EventType] = true
		}
		assert.True(t, seen[domain.EventTypeCreated])
		assert.True(t, seen[domain.EventTypeNamespaceLocked])
		assert.True(t, seen[domain.EventTypeNamespaceUnlocked])
	})
}

// Sanity: a wrapped storage.ErrResourceNotFound from a missing namespace lookup
// during Get should not be confused with a different sentinel.
func TestRepository_Get_NotFound_NotErrAlreadyExists(t *testing.T) {
	t.Parallel()

	repo, _, _ := newRepo(t)

	_, err := repo.Get(t.Context(), "/x", "ns")
	require.Error(t, err)
	require.NotErrorIs(t, err, storage.ErrResourceAlreadyExists)
}

func TestRepository_WithTx_RollbackDiscardsWrites(t *testing.T) {
	t.Parallel()

	repo, nsr, mgr := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	err := mgr.WithTx(ctx, func(ctx context.Context) error {
		if err := repo.Create(ctx, newTestConfig("ns", "/a", "v")); err != nil {
			return err
		}

		return assert.AnError
	})
	require.ErrorIs(t, err, assert.AnError)

	_, err = repo.Get(ctx, "/a", "ns")
	require.ErrorIs(t, err, storage.ErrResourceNotFound)
}
