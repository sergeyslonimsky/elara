package schema_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	schemarepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/schema"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

const testJSONSchema = `{"type": "object", "properties": {"host": {"type": "string"}}}`

func newRepo(t *testing.T) *schemarepo.Repository {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	mgr := pkgbbolt.NewManager(store.DB())

	return schemarepo.NewRepository(mgr)
}

func newTestAttachment(namespace, pathPattern string) *domain.SchemaAttachment {
	return &domain.SchemaAttachment{
		ID:          namespace + "/" + pathPattern,
		Namespace:   namespace,
		PathPattern: pathPattern,
		JSONSchema:  testJSONSchema,
	}
}

func TestRepository_Attach_Get_RoundTrip(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	ctx := t.Context()

	att := newTestAttachment("ns1", "app/*")

	require.NoError(t, repo.Attach(ctx, att))
	assert.False(t, att.CreatedAt.IsZero(), "CreatedAt should be populated on attach")

	got, err := repo.Get(ctx, "ns1", "app/*")
	require.NoError(t, err)
	assert.Equal(t, att.Namespace, got.Namespace)
	assert.Equal(t, att.PathPattern, got.PathPattern)
	assert.Equal(t, att.JSONSchema, got.JSONSchema)
	assert.WithinDuration(t, att.CreatedAt, got.CreatedAt, time.Second)
}

func TestRepository_Attach_UpsertPreservesCreatedAt(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	ctx := t.Context()

	original := newTestAttachment("ns1", "app/*")
	original.CreatedAt = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.Attach(ctx, original))

	updated := newTestAttachment("ns1", "app/*")
	updated.JSONSchema = `{"type": "object", "properties": {"port": {"type": "number"}}}`
	require.NoError(t, repo.Attach(ctx, updated))

	got, err := repo.Get(ctx, "ns1", "app/*")
	require.NoError(t, err)
	assert.Equal(t, updated.JSONSchema, got.JSONSchema)
	assert.WithinDuration(t, original.CreatedAt, got.CreatedAt, time.Second)
}

func TestRepository_Detach(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setup       func(t *testing.T) *schemarepo.Repository
		namespace   string
		pathPattern string
		errIs       error
	}{
		{
			name: "success removes attachment",
			setup: func(t *testing.T) *schemarepo.Repository {
				t.Helper()
				repo := newRepo(t)
				require.NoError(t, repo.Attach(t.Context(), newTestAttachment("ns1", "app/*")))

				return repo
			},
			namespace:   "ns1",
			pathPattern: "app/*",
		},
		{
			name: "missing returns ErrResourceNotFound",
			setup: func(t *testing.T) *schemarepo.Repository {
				t.Helper()

				return newRepo(t)
			},
			namespace:   "ns1",
			pathPattern: "nonexistent/*",
			errIs:       storage.ErrResourceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := tt.setup(t)
			ctx := t.Context()

			err := repo.Detach(ctx, tt.namespace, tt.pathPattern)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			require.NoError(t, err)

			_, getErr := repo.Get(ctx, tt.namespace, tt.pathPattern)
			require.ErrorIs(t, getErr, storage.ErrResourceNotFound)
		})
	}
}

func TestRepository_Get_NotFound(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)

	got, err := repo.Get(t.Context(), "ns1", "missing/*")
	require.ErrorIs(t, err, storage.ErrResourceNotFound)
	assert.Nil(t, got)
}

func TestRepository_List(t *testing.T) {
	t.Parallel()

	seed := func(t *testing.T) *schemarepo.Repository {
		t.Helper()
		repo := newRepo(t)
		ctx := t.Context()

		require.NoError(t, repo.Attach(ctx, newTestAttachment("ns1", "app/a")))
		require.NoError(t, repo.Attach(ctx, newTestAttachment("ns1", "app/b")))
		require.NoError(t, repo.Attach(ctx, newTestAttachment("ns2", "app/c")))

		return repo
	}

	tests := []struct {
		name      string
		namespace string
		wantLen   int
	}{
		{
			name:      "filters by namespace",
			namespace: "ns1",
			wantLen:   2,
		},
		{
			name:      "other namespace returns one",
			namespace: "ns2",
			wantLen:   1,
		},
		{
			name:      "empty namespace returns empty",
			namespace: "ns-missing",
			wantLen:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := seed(t)

			got, err := repo.List(t.Context(), tt.namespace)
			require.NoError(t, err)
			assert.Len(t, got, tt.wantLen)
			for _, a := range got {
				assert.Equal(t, tt.namespace, a.Namespace)
			}
		})
	}
}
