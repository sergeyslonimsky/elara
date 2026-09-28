package lease_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	leaserepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/lease"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

func newRepo(t *testing.T) (*leaserepo.Repository, pkgbbolt.Manager) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "elara.db")
	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	mgr := pkgbbolt.NewManager(store.DB())

	return leaserepo.NewRepository(mgr), mgr
}

func newLease(t *testing.T, id int64, expiresAt time.Time) *domain.Lease {
	t.Helper()

	const ttl = 30 * time.Second

	return &domain.Lease{
		ID:        id,
		TTL:       ttl,
		GrantedAt: expiresAt.Add(-ttl),
		ExpiresAt: expiresAt,
	}
}

func TestRepository_GrantGet(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Millisecond)

	tests := []struct {
		name  string
		lease *domain.Lease
	}{
		{
			name:  "round-trips a live lease",
			lease: newLease(t, 1, now.Add(time.Minute)),
		},
		{
			name:  "round-trips an already-expired lease",
			lease: newLease(t, 2, now.Add(-time.Minute)),
		},
		{
			name:  "round-trips a large id",
			lease: newLease(t, 1<<62, now.Add(time.Minute)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, _ := newRepo(t)
			ctx := t.Context()

			require.NoError(t, repo.Grant(ctx, tt.lease))

			got, err := repo.Get(ctx, tt.lease.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.lease.ID, got.ID)
			assert.Equal(t, tt.lease.TTL, got.TTL)
			assert.True(t, tt.lease.ExpiresAt.Equal(got.ExpiresAt))
			assert.True(t, tt.lease.GrantedAt.Equal(got.GrantedAt))
		})
	}
}

func TestRepository_GrantGetErrors(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	t.Run("granting a taken id reports already-exists", func(t *testing.T) {
		t.Parallel()

		repo, _ := newRepo(t)
		ctx := t.Context()

		require.NoError(t, repo.Grant(ctx, newLease(t, 7, now.Add(time.Minute))))

		err := repo.Grant(ctx, newLease(t, 7, now.Add(time.Hour)))
		require.ErrorIs(t, err, storage.ErrResourceAlreadyExists)
	})

	t.Run("getting a missing lease reports not-found", func(t *testing.T) {
		t.Parallel()

		repo, _ := newRepo(t)

		_, err := repo.Get(t.Context(), 404)
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_Renew(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Millisecond)

	t.Run("moves the expiry", func(t *testing.T) {
		t.Parallel()

		repo, _ := newRepo(t)
		ctx := t.Context()
		newExpiry := now.Add(time.Hour)

		require.NoError(t, repo.Grant(ctx, newLease(t, 1, now.Add(time.Minute))))
		require.NoError(t, repo.Renew(ctx, 1, newExpiry))

		got, err := repo.Get(ctx, 1)
		require.NoError(t, err)
		assert.True(t, newExpiry.Equal(got.ExpiresAt))
	})

	t.Run("drops the stale expiry-index entry", func(t *testing.T) {
		t.Parallel()

		// The regression this guards: the index key encodes the expiry, so a
		// renewal writes a *new* entry rather than overwriting the old one. A
		// surviving stale entry makes the sweep revoke a lease that was renewed
		// on time.
		repo, _ := newRepo(t)
		ctx := t.Context()
		oldExpiry := now.Add(time.Minute)

		require.NoError(t, repo.Grant(ctx, newLease(t, 1, oldExpiry)))
		require.NoError(t, repo.Renew(ctx, 1, now.Add(time.Hour)))

		due, err := repo.ExpiredBefore(ctx, oldExpiry.Add(time.Second), 0)
		require.NoError(t, err)
		assert.Empty(t, due, "renewed lease must not remain reachable under its old due time")
	})

	t.Run("renewing a missing lease reports not-found", func(t *testing.T) {
		t.Parallel()

		repo, _ := newRepo(t)

		err := repo.Renew(t.Context(), 404, now)
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_Keys(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	t.Run("attach then list", func(t *testing.T) {
		t.Parallel()

		repo, _ := newRepo(t)
		ctx := t.Context()
		refs := []domain.KeyRef{
			{Namespace: "prod", Path: "/lock/leader"},
			{Namespace: "prod", Path: "/lock/worker"},
			{Namespace: "staging", Path: "/lock/leader"},
		}

		require.NoError(t, repo.Grant(ctx, newLease(t, 1, now.Add(time.Minute))))

		for _, ref := range refs {
			require.NoError(t, repo.AttachKey(ctx, 1, ref))
		}

		got, err := repo.KeysOf(ctx, 1)
		require.NoError(t, err)
		assert.ElementsMatch(t, refs, got)
	})

	t.Run("keys of one lease are not visible on another", func(t *testing.T) {
		t.Parallel()

		repo, _ := newRepo(t)
		ctx := t.Context()

		require.NoError(t, repo.Grant(ctx, newLease(t, 1, now.Add(time.Minute))))
		require.NoError(t, repo.Grant(ctx, newLease(t, 2, now.Add(time.Minute))))
		require.NoError(t, repo.AttachKey(ctx, 1, domain.KeyRef{Namespace: "prod", Path: "/a.json"}))

		got, err := repo.KeysOf(ctx, 2)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("detach removes the claim and is idempotent", func(t *testing.T) {
		t.Parallel()

		repo, _ := newRepo(t)
		ctx := t.Context()
		ref := domain.KeyRef{Namespace: "prod", Path: "/a.json"}

		require.NoError(t, repo.Grant(ctx, newLease(t, 1, now.Add(time.Minute))))
		require.NoError(t, repo.AttachKey(ctx, 1, ref))
		require.NoError(t, repo.DetachKey(ctx, 1, ref))
		require.NoError(t, repo.DetachKey(ctx, 1, ref), "detaching twice is a no-op, not an error")

		got, err := repo.KeysOf(ctx, 1)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestRepository_Revoke(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	t.Run("returns the attached keys and clears all three buckets", func(t *testing.T) {
		t.Parallel()

		repo, _ := newRepo(t)
		ctx := t.Context()
		expiresAt := now.Add(time.Minute)
		refs := []domain.KeyRef{
			{Namespace: "prod", Path: "/lock/leader"},
			{Namespace: "prod", Path: "/lock/worker"},
		}

		require.NoError(t, repo.Grant(ctx, newLease(t, 1, expiresAt)))

		for _, ref := range refs {
			require.NoError(t, repo.AttachKey(ctx, 1, ref))
		}

		got, err := repo.Revoke(ctx, 1)
		require.NoError(t, err)
		assert.ElementsMatch(t, refs, got)

		_, err = repo.Get(ctx, 1)
		require.ErrorIs(t, err, storage.ErrResourceNotFound)

		keys, err := repo.KeysOf(ctx, 1)
		require.NoError(t, err)
		assert.Empty(t, keys)

		due, err := repo.ExpiredBefore(ctx, expiresAt.Add(time.Hour), 0)
		require.NoError(t, err)
		assert.Empty(t, due, "expiry index entry must go with the lease")
	})

	t.Run("revoking a missing lease reports not-found", func(t *testing.T) {
		t.Parallel()

		repo, _ := newRepo(t)

		_, err := repo.Revoke(t.Context(), 404)
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_ExpiredBefore(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Millisecond)

	tests := []struct {
		name    string
		expiry  map[int64]time.Time
		cutoff  time.Time
		limit   int
		wantIDs []int64
	}{
		{
			name: "returns due leases soonest first",
			expiry: map[int64]time.Time{
				1: now.Add(3 * time.Second),
				2: now.Add(time.Second),
				3: now.Add(2 * time.Second),
			},
			cutoff:  now.Add(10 * time.Second),
			limit:   0,
			wantIDs: []int64{2, 3, 1},
		},
		{
			name: "stops at the first lease that is not due",
			expiry: map[int64]time.Time{
				1: now.Add(time.Second),
				2: now.Add(time.Hour),
			},
			cutoff:  now.Add(time.Minute),
			limit:   0,
			wantIDs: []int64{1},
		},
		{
			// Strictly before, matching domain.Lease.IsExpired: a lease is alive
			// at exactly its expiry instant.
			name:    "a lease due exactly at the cutoff is not returned",
			expiry:  map[int64]time.Time{1: now},
			cutoff:  now,
			limit:   0,
			wantIDs: nil,
		},
		{
			name: "limit caps the batch",
			expiry: map[int64]time.Time{
				1: now.Add(time.Second),
				2: now.Add(2 * time.Second),
				3: now.Add(3 * time.Second),
			},
			cutoff:  now.Add(time.Minute),
			limit:   2,
			wantIDs: []int64{1, 2},
		},
		{
			name:    "no leases at all",
			expiry:  nil,
			cutoff:  now,
			limit:   0,
			wantIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, _ := newRepo(t)
			ctx := t.Context()

			for id, expiresAt := range tt.expiry {
				require.NoError(t, repo.Grant(ctx, newLease(t, id, expiresAt)))
			}

			got, err := repo.ExpiredBefore(ctx, tt.cutoff, tt.limit)
			require.NoError(t, err)
			assert.Equal(t, tt.wantIDs, got)
		})
	}
}

func TestRepository_List(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	repo, _ := newRepo(t)
	ctx := t.Context()

	require.NoError(t, repo.Grant(ctx, newLease(t, 2, now.Add(time.Minute))))
	require.NoError(t, repo.Grant(ctx, newLease(t, 1, now.Add(time.Hour))))

	got, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, got, 2)
	// Key order, and the primary key is the big-endian ID.
	assert.Equal(t, int64(1), got[0].ID)
	assert.Equal(t, int64(2), got[1].ID)
}

func TestRepository_GrantRollsBackWithTheTransaction(t *testing.T) {
	t.Parallel()

	// A lease record without its expiry-index entry would never expire, so the
	// two writes have to land together. The repository does not open its own
	// transaction; this is what the caller's WithTx buys.
	repo, mgr := newRepo(t)
	ctx := t.Context()
	expiresAt := time.Now().UTC().Add(time.Minute)
	sentinel := errors.New("rollback")

	err := mgr.WithTx(ctx, func(ctx context.Context) error {
		if err := repo.Grant(ctx, newLease(t, 1, expiresAt)); err != nil {
			return err
		}

		return sentinel
	})
	require.ErrorIs(t, err, sentinel)

	_, err = repo.Get(ctx, 1)
	require.ErrorIs(t, err, storage.ErrResourceNotFound)

	due, err := repo.ExpiredBefore(ctx, expiresAt.Add(time.Hour), 0)
	require.NoError(t, err)
	assert.Empty(t, due)
}
