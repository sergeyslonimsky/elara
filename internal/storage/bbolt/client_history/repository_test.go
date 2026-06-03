package client_history_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	clienthistoryrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/client_history"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

func newRepo(t *testing.T) *clienthistoryrepo.Repository {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	return clienthistoryrepo.NewRepository(pkgbbolt.NewManager(store.DB()))
}

func makeSnap(id, clientName, k8sNamespace string, disconnectedAt time.Time) *domain.Client {
	return &domain.Client{
		ID:             id,
		PeerAddress:    "10.0.0.1:1234",
		ClientName:     clientName,
		K8sNamespace:   k8sNamespace,
		ConnectedAt:    disconnectedAt.Add(-5 * time.Minute),
		DisconnectedAt: new(disconnectedAt),
		LastActivityAt: disconnectedAt,
		ActiveWatches:  0,
		RequestCounts:  map[string]int64{"Put": 3, "Range": 1},
		ErrorCount:     0,
	}
}

func TestRepository_Save(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		setup  func(t *testing.T) (*clienthistoryrepo.Repository, *domain.Client)
		assert func(t *testing.T, repo *clienthistoryrepo.Repository)
	}{
		{
			name: "success persists snapshot",
			setup: func(t *testing.T) (*clienthistoryrepo.Repository, *domain.Client) {
				t.Helper()
				repo := newRepo(t)

				return repo, makeSnap("a", "svc", "ns", time.Now())
			},
			assert: func(t *testing.T, repo *clienthistoryrepo.Repository) {
				t.Helper()

				got, err := repo.List(t.Context(), 0)
				require.NoError(t, err)
				assert.Len(t, got, 1)
			},
		},
		{
			name: "same-nano collision: both persisted via ID suffix",
			setup: func(t *testing.T) (*clienthistoryrepo.Repository, *domain.Client) {
				t.Helper()
				repo := newRepo(t)

				t0 := time.Now()
				require.NoError(t, repo.Save(t.Context(), makeSnap("a", "svc", "ns", t0)))

				return repo, makeSnap("b", "svc", "ns", t0)
			},
			assert: func(t *testing.T, repo *clienthistoryrepo.Repository) {
				t.Helper()

				got, err := repo.List(t.Context(), 0)
				require.NoError(t, err)
				assert.Len(t, got, 2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, snap := tt.setup(t)

			err := repo.Save(t.Context(), snap)
			require.NoError(t, err)

			tt.assert(t, repo)
		})
	}
}

func TestRepository_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T) *clienthistoryrepo.Repository
		limit   int
		wantIDs []string
	}{
		{
			name: "newest first, no limit",
			setup: func(t *testing.T) *clienthistoryrepo.Repository {
				t.Helper()
				repo := newRepo(t)

				t0 := time.Now().Truncate(time.Second)
				require.NoError(t, repo.Save(t.Context(), makeSnap("a", "svc", "ns", t0)))
				require.NoError(t, repo.Save(t.Context(), makeSnap("b", "svc", "ns", t0.Add(time.Second))))
				require.NoError(t, repo.Save(t.Context(), makeSnap("c", "svc", "ns", t0.Add(2*time.Second))))

				return repo
			},
			limit:   0,
			wantIDs: []string{"c", "b", "a"},
		},
		{
			name: "respects limit",
			setup: func(t *testing.T) *clienthistoryrepo.Repository {
				t.Helper()
				repo := newRepo(t)

				t0 := time.Now()
				for i := range 5 {
					require.NoError(
						t,
						repo.Save(t.Context(), makeSnap("c", "svc", "ns", t0.Add(time.Duration(i)*time.Second))),
					)
				}

				return repo
			},
			limit:   2,
			wantIDs: nil, // assert only length
		},
		{
			name: "empty bucket returns nil",
			setup: func(t *testing.T) *clienthistoryrepo.Repository {
				t.Helper()

				return newRepo(t)
			},
			limit:   0,
			wantIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := tt.setup(t)

			got, err := repo.List(t.Context(), tt.limit)
			require.NoError(t, err)

			if tt.wantIDs != nil {
				require.Len(t, got, len(tt.wantIDs))
				for i, want := range tt.wantIDs {
					assert.Equal(t, want, got[i].ID)
				}

				return
			}

			if tt.limit > 0 {
				assert.Len(t, got, tt.limit)
			}
		})
	}
}

func TestRepository_ListByClient(t *testing.T) {
	t.Parallel()

	seed := func(t *testing.T) *clienthistoryrepo.Repository {
		t.Helper()
		repo := newRepo(t)
		t0 := time.Now()

		snaps := []*domain.Client{
			makeSnap("a", "order-service", "production", t0),
			makeSnap("b", "order-service", "staging", t0.Add(time.Second)),
			makeSnap("c", "payment-service", "production", t0.Add(2*time.Second)),
			makeSnap("d", "order-service", "production", t0.Add(3*time.Second)),
		}
		for _, s := range snaps {
			require.NoError(t, repo.Save(t.Context(), s))
		}

		return repo
	}

	tests := []struct {
		name         string
		clientName   string
		k8sNamespace string
		limit        int
		wantIDs      []string
	}{
		{
			name:         "matches name+namespace, newest first",
			clientName:   "order-service",
			k8sNamespace: "production",
			limit:        0,
			wantIDs:      []string{"d", "a"},
		},
		{
			name:         "respects limit",
			clientName:   "order-service",
			k8sNamespace: "production",
			limit:        1,
			wantIDs:      []string{"d"},
		},
		{
			name:         "no matches returns empty",
			clientName:   "ghost",
			k8sNamespace: "nowhere",
			limit:        0,
			wantIDs:      nil,
		},
		{
			name:         "empty namespace must match exactly",
			clientName:   "order-service",
			k8sNamespace: "",
			limit:        0,
			wantIDs:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := seed(t)

			got, err := repo.ListByClient(t.Context(), tt.clientName, tt.k8sNamespace, tt.limit)
			require.NoError(t, err)

			require.Len(t, got, len(tt.wantIDs))
			for i, want := range tt.wantIDs {
				assert.Equal(t, want, got[i].ID)
			}
		})
	}
}

func TestRepository_Count(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(t *testing.T) *clienthistoryrepo.Repository
		want  int
	}{
		{
			name: "empty",
			setup: func(t *testing.T) *clienthistoryrepo.Repository {
				t.Helper()

				return newRepo(t)
			},
			want: 0,
		},
		{
			name: "two entries",
			setup: func(t *testing.T) *clienthistoryrepo.Repository {
				t.Helper()
				repo := newRepo(t)

				require.NoError(t, repo.Save(t.Context(), makeSnap("a", "svc", "ns", time.Now())))
				require.NoError(t, repo.Save(t.Context(), makeSnap("b", "svc", "ns", time.Now().Add(time.Second))))

				return repo
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := tt.setup(t)

			got, err := repo.Count(t.Context())
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRepository_DeleteOldest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setup       func(t *testing.T) *clienthistoryrepo.Repository
		n           int
		wantDeleted int
		wantRemain  int
	}{
		{
			name: "deletes oldest n keeping newest",
			setup: func(t *testing.T) *clienthistoryrepo.Repository {
				t.Helper()
				repo := newRepo(t)

				t0 := time.Now()
				for i := range 5 {
					require.NoError(
						t,
						repo.Save(t.Context(), makeSnap("c", "svc", "ns", t0.Add(time.Duration(i)*time.Second))),
					)
				}

				return repo
			},
			n:           3,
			wantDeleted: 3,
			wantRemain:  2,
		},
		{
			name: "n larger than available deletes all",
			setup: func(t *testing.T) *clienthistoryrepo.Repository {
				t.Helper()
				repo := newRepo(t)
				require.NoError(t, repo.Save(t.Context(), makeSnap("a", "svc", "ns", time.Now())))

				return repo
			},
			n:           10,
			wantDeleted: 1,
			wantRemain:  0,
		},
		{
			name: "n zero is no-op",
			setup: func(t *testing.T) *clienthistoryrepo.Repository {
				t.Helper()
				repo := newRepo(t)
				require.NoError(t, repo.Save(t.Context(), makeSnap("a", "svc", "ns", time.Now())))

				return repo
			},
			n:           0,
			wantDeleted: 0,
			wantRemain:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := tt.setup(t)

			deleted, err := repo.DeleteOldest(t.Context(), tt.n)
			require.NoError(t, err)
			assert.Equal(t, tt.wantDeleted, deleted)

			remain, err := repo.Count(t.Context())
			require.NoError(t, err)
			assert.Equal(t, tt.wantRemain, remain)
		})
	}
}

func TestRepository_DeleteOlderThan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setup         func(t *testing.T) (*clienthistoryrepo.Repository, time.Time)
		wantDeleted   int
		wantRemainIDs []string
	}{
		{
			name: "deletes entries strictly older than cutoff",
			setup: func(t *testing.T) (*clienthistoryrepo.Repository, time.Time) {
				t.Helper()
				repo := newRepo(t)
				t0 := time.Now()

				require.NoError(t, repo.Save(t.Context(), makeSnap("old1", "svc", "ns", t0.Add(-2*time.Hour))))
				require.NoError(t, repo.Save(t.Context(), makeSnap("old2", "svc", "ns", t0.Add(-1*time.Hour))))
				require.NoError(t, repo.Save(t.Context(), makeSnap("recent", "svc", "ns", t0.Add(-5*time.Minute))))

				return repo, t0.Add(-30 * time.Minute)
			},
			wantDeleted:   2,
			wantRemainIDs: []string{"recent"},
		},
		{
			name: "cutoff older than all entries deletes nothing",
			setup: func(t *testing.T) (*clienthistoryrepo.Repository, time.Time) {
				t.Helper()
				repo := newRepo(t)
				t0 := time.Now()
				require.NoError(t, repo.Save(t.Context(), makeSnap("a", "svc", "ns", t0)))

				return repo, t0.Add(-1 * time.Hour)
			},
			wantDeleted:   0,
			wantRemainIDs: []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, cutoff := tt.setup(t)

			deleted, err := repo.DeleteOlderThan(t.Context(), cutoff)
			require.NoError(t, err)
			assert.Equal(t, tt.wantDeleted, deleted)

			got, err := repo.List(t.Context(), 0)
			require.NoError(t, err)
			require.Len(t, got, len(tt.wantRemainIDs))
			for i, want := range tt.wantRemainIDs {
				assert.Equal(t, want, got[i].ID)
			}
		})
	}
}

func TestRepository_PreservesAllFields(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)

	d := time.Now().Truncate(time.Second).UTC()
	in := &domain.Client{
		ID:             "conn-1",
		PeerAddress:    "10.0.0.5:54321",
		UserAgent:      "ua",
		ClientName:     "order-service",
		ClientVersion:  "1.2.3",
		K8sNamespace:   "production",
		K8sPod:         "pod-1",
		K8sNode:        "node-1",
		InstanceID:     "instance-1",
		ConnectedAt:    d.Add(-time.Hour).UTC(),
		DisconnectedAt: &d,
		LastActivityAt: d.Add(-time.Minute).UTC(),
		ActiveWatches:  3,
		RequestCounts:  map[string]int64{"Put": 10},
		ErrorCount:     2,
	}

	require.NoError(t, repo.Save(t.Context(), in))

	got, err := repo.List(t.Context(), 1)
	require.NoError(t, err)
	require.Len(t, got, 1)

	out := got[0]
	assert.Equal(t, in.ID, out.ID)
	assert.Equal(t, in.PeerAddress, out.PeerAddress)
	assert.Equal(t, in.UserAgent, out.UserAgent)
	assert.Equal(t, in.ClientName, out.ClientName)
	assert.Equal(t, in.ClientVersion, out.ClientVersion)
	assert.Equal(t, in.K8sNamespace, out.K8sNamespace)
	assert.Equal(t, in.K8sPod, out.K8sPod)
	assert.Equal(t, in.K8sNode, out.K8sNode)
	assert.Equal(t, in.InstanceID, out.InstanceID)
	assert.True(t, in.ConnectedAt.Equal(out.ConnectedAt))
	assert.True(t, in.DisconnectedAt.Equal(*out.DisconnectedAt))
	assert.True(t, in.LastActivityAt.Equal(out.LastActivityAt))
	assert.Equal(t, in.ActiveWatches, out.ActiveWatches)
	assert.Equal(t, in.RequestCounts, out.RequestCounts)
	assert.Equal(t, in.ErrorCount, out.ErrorCount)
}
