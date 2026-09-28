package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestRepository_PutKey_LeaseAssignment(t *testing.T) {
	t.Parallel()

	const existingLease = int64(111)

	tests := []struct {
		name      string
		seed      bool // write the key first, attached to existingLease
		requested domain.LeaseAssignment
		want      int64
	}{
		{
			name:      "new key with no lease requested has none",
			seed:      false,
			requested: domain.LeaseAssignment{},
			want:      0,
		},
		{
			name:      "new key attaches to the requested lease",
			seed:      false,
			requested: domain.LeaseAssignment{ID: 42},
			want:      42,
		},
		{
			name:      "overwrite with ignore_lease keeps the existing lease",
			seed:      true,
			requested: domain.LeaseAssignment{Ignore: true},
			want:      existingLease,
		},
		{
			// Ignore wins over ID, the way it does on the wire: a client that
			// sets both is asking to keep what is there.
			name:      "ignore_lease wins over a requested id",
			seed:      true,
			requested: domain.LeaseAssignment{ID: 999, Ignore: true},
			want:      existingLease,
		},
		{
			name:      "overwrite with zero detaches",
			seed:      true,
			requested: domain.LeaseAssignment{},
			want:      0,
		},
		{
			name:      "overwrite with another lease reattaches",
			seed:      true,
			requested: domain.LeaseAssignment{ID: 222},
			want:      222,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, nsr, _ := newRepo(t)
			ctx := t.Context()
			seedNamespace(t, nsr, "ns")

			if tt.seed {
				_, _, err := repo.PutKey(
					ctx, "ns", "/a", []byte("v1"),
					domain.LeaseAssignment{ID: existingLease},
				)
				require.NoError(t, err)
			}

			_, _, err := repo.PutKey(ctx, "ns", "/a", []byte("v2"), tt.requested)
			require.NoError(t, err)

			results, _, err := repo.RangeQuery(ctx, "ns", "/a", "", "", 0, 0, false)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, tt.want, results[0].Lease)
		})
	}
}

func TestRepository_PutKey_ReturnsPreviousLease(t *testing.T) {
	t.Parallel()

	// The caller moves the lease-to-keys index by the previous attachment, so the
	// prev KV has to carry it.
	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	_, _, err := repo.PutKey(ctx, "ns", "/a", []byte("v1"), domain.LeaseAssignment{ID: 77})
	require.NoError(t, err)

	prev, _, err := repo.PutKey(ctx, "ns", "/a", []byte("v2"), domain.LeaseAssignment{})
	require.NoError(t, err)
	require.NotNil(t, prev)
	assert.Equal(t, int64(77), prev.Lease)
}

func TestRepository_DeleteKeys_OneRevisionForTheWholeSet(t *testing.T) {
	t.Parallel()

	// etcd removes every key of a revoked lease in a single revision. A loop over
	// DeleteRangeKeys would spend one revision per key, which watch clients see.
	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	for _, path := range []string{"/a", "/b", "/c"} {
		_, _, err := repo.PutKey(ctx, "ns", path, []byte("v"), domain.LeaseAssignment{})
		require.NoError(t, err)
	}

	before, err := repo.CurrentRevisionValue(ctx)
	require.NoError(t, err)

	deleted, rev, err := repo.DeleteKeys(ctx, []domain.KeyRef{
		{Namespace: "ns", Path: "/a"},
		{Namespace: "ns", Path: "/c"},
	}, false)
	require.NoError(t, err)
	require.Len(t, deleted, 2)
	assert.Equal(t, before+1, rev, "the whole set costs exactly one revision")

	results, _, err := repo.RangeQuery(ctx, "ns", "/a", "\x00", "", 0, 0, false)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "/b", results[0].Path, "only the requested keys are gone")
}

func TestRepository_DeleteKeys_SkipsMissingKeys(t *testing.T) {
	t.Parallel()

	// The expiry sweep can race a client that deleted the key itself.
	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	_, _, err := repo.PutKey(ctx, "ns", "/a", []byte("v"), domain.LeaseAssignment{})
	require.NoError(t, err)

	deleted, _, err := repo.DeleteKeys(ctx, []domain.KeyRef{
		{Namespace: "ns", Path: "/a"},
		{Namespace: "ns", Path: "/never-existed"},
	}, false)
	require.NoError(t, err)
	require.Len(t, deleted, 1)
	assert.Equal(t, "/a", deleted[0].Path)
}

func TestRepository_DeleteKeys_NothingToDelete(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	deleted, rev, err := repo.DeleteKeys(ctx, []domain.KeyRef{
		{Namespace: "ns", Path: "/never-existed"},
	}, false)
	require.NoError(t, err)
	assert.Empty(t, deleted)
	assert.Zero(t, rev, "an empty delete must not burn a revision")
}

func TestRepository_DeleteKeys_ReturnsPrevWhenAsked(t *testing.T) {
	t.Parallel()

	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	_, _, err := repo.PutKey(ctx, "ns", "/a", []byte("v1"), domain.LeaseAssignment{ID: 5})
	require.NoError(t, err)

	deleted, _, err := repo.DeleteKeys(ctx, []domain.KeyRef{{Namespace: "ns", Path: "/a"}}, true)
	require.NoError(t, err)
	require.Len(t, deleted, 1)
	assert.Equal(t, []byte("v1"), deleted[0].Value)
	assert.Equal(t, int64(1), deleted[0].Version)
	assert.Equal(t, int64(5), deleted[0].Lease)
}

func TestRepository_DeletedKeysCarryTheirLeaseWithoutPrevKv(t *testing.T) {
	t.Parallel()

	// Lease travels regardless of returnPrev: the caller needs it to drop the
	// lease's claim, which has nothing to do with answering prev_kv.
	repo, nsr, _ := newRepo(t)
	ctx := t.Context()
	seedNamespace(t, nsr, "ns")

	_, _, err := repo.PutKey(ctx, "ns", "/a", []byte("v"), domain.LeaseAssignment{ID: 9})
	require.NoError(t, err)

	viaSet, _, err := repo.DeleteKeys(ctx, []domain.KeyRef{{Namespace: "ns", Path: "/a"}}, false)
	require.NoError(t, err)
	require.Len(t, viaSet, 1)
	assert.Equal(t, int64(9), viaSet[0].Lease)
	assert.Nil(t, viaSet[0].Value, "returnPrev was false, so no value")

	_, _, err = repo.PutKey(ctx, "ns", "/b", []byte("v"), domain.LeaseAssignment{ID: 9})
	require.NoError(t, err)

	viaRange, _, err := repo.DeleteRangeKeys(ctx, "ns", "/b", "", "", false)
	require.NoError(t, err)
	require.Len(t, viaRange, 1)
	assert.Equal(t, int64(9), viaRange[0].Lease)
}
