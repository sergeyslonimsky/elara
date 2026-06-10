package webhook_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	webhookrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/webhook"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

func newRepo(t *testing.T) (*webhookrepo.Repository, pkgbbolt.Manager) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	mgr := pkgbbolt.NewManager(store.DB())

	return webhookrepo.NewRepository(mgr), mgr
}

func newTestWebhook(url string) *domain.Webhook {
	now := time.Now()

	return &domain.Webhook{
		ID:        uuid.NewString(),
		URL:       url,
		Events:    []domain.WebhookEventType{domain.WebhookEventCreated, domain.WebhookEventUpdated},
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestRepository_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T) (*webhookrepo.Repository, *domain.Webhook)
		errIs   error
		wantErr string
	}{
		{
			name: "success",
			setup: func(t *testing.T) (*webhookrepo.Repository, *domain.Webhook) {
				t.Helper()
				repo, _ := newRepo(t)

				return repo, newTestWebhook("https://example.com/hook")
			},
		},
		{
			name: "duplicate id returns ErrResourceAlreadyExists",
			setup: func(t *testing.T) (*webhookrepo.Repository, *domain.Webhook) {
				t.Helper()
				repo, _ := newRepo(t)

				existing := newTestWebhook("https://example.com/hook")
				require.NoError(t, repo.Create(t.Context(), existing))

				dup := newTestWebhook("https://other.example.com/hook")
				dup.ID = existing.ID

				return repo, dup
			},
			errIs: storage.ErrResourceAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, w := tt.setup(t)

			err := repo.Create(t.Context(), w)

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
		setup   func(t *testing.T) (*webhookrepo.Repository, string, *domain.Webhook)
		errIs   error
		wantErr string
	}{
		{
			name: "success",
			setup: func(t *testing.T) (*webhookrepo.Repository, string, *domain.Webhook) {
				t.Helper()
				repo, _ := newRepo(t)
				w := newTestWebhook("https://example.com/hook")
				require.NoError(t, repo.Create(t.Context(), w))

				return repo, w.ID, w
			},
		},
		{
			name: "missing id returns ErrResourceNotFound",
			setup: func(t *testing.T) (*webhookrepo.Repository, string, *domain.Webhook) {
				t.Helper()
				repo, _ := newRepo(t)

				return repo, "nonexistent-id", nil
			},
			errIs: storage.ErrResourceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, id, want := tt.setup(t)

			got, err := repo.Get(t.Context(), id)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, want.ID, got.ID)
			assert.Equal(t, want.URL, got.URL)
			assert.Equal(t, want.Events, got.Events)
			assert.Equal(t, want.Enabled, got.Enabled)
		})
	}
}

func TestRepository_Update(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T) (*webhookrepo.Repository, *domain.Webhook)
		assert  func(t *testing.T, repo *webhookrepo.Repository, w *domain.Webhook)
		errIs   error
		wantErr string
	}{
		{
			name: "success persists new fields",
			setup: func(t *testing.T) (*webhookrepo.Repository, *domain.Webhook) {
				t.Helper()
				repo, _ := newRepo(t)

				w := newTestWebhook("https://example.com/hook")
				require.NoError(t, repo.Create(t.Context(), w))

				w.URL = "https://updated.example.com/hook"
				w.Enabled = false

				return repo, w
			},
			assert: func(t *testing.T, repo *webhookrepo.Repository, w *domain.Webhook) {
				t.Helper()

				got, err := repo.Get(t.Context(), w.ID)
				require.NoError(t, err)
				assert.Equal(t, "https://updated.example.com/hook", got.URL)
				assert.False(t, got.Enabled)
			},
		},
		{
			name: "missing id returns ErrResourceNotFound",
			setup: func(t *testing.T) (*webhookrepo.Repository, *domain.Webhook) {
				t.Helper()
				repo, _ := newRepo(t)

				w := newTestWebhook("https://example.com/hook")
				w.ID = "nonexistent-id"

				return repo, w
			},
			errIs: storage.ErrResourceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, w := tt.setup(t)

			err := repo.Update(t.Context(), w)

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
				tt.assert(t, repo, w)
			}
		})
	}
}

func TestRepository_Delete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T) (*webhookrepo.Repository, string)
		errIs   error
		wantErr string
	}{
		{
			name: "success",
			setup: func(t *testing.T) (*webhookrepo.Repository, string) {
				t.Helper()
				repo, _ := newRepo(t)

				w := newTestWebhook("https://example.com/hook")
				require.NoError(t, repo.Create(t.Context(), w))

				return repo, w.ID
			},
		},
		{
			name: "missing id returns ErrResourceNotFound",
			setup: func(t *testing.T) (*webhookrepo.Repository, string) {
				t.Helper()
				repo, _ := newRepo(t)

				return repo, "nonexistent-id"
			},
			errIs: storage.ErrResourceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, id := tt.setup(t)

			err := repo.Delete(t.Context(), id)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)

			_, getErr := repo.Get(t.Context(), id)
			require.ErrorIs(t, getErr, storage.ErrResourceNotFound)
		})
	}
}

func TestRepository_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(t *testing.T) *webhookrepo.Repository
		want  int
	}{
		{
			name: "empty bucket returns empty slice",
			setup: func(t *testing.T) *webhookrepo.Repository {
				t.Helper()
				repo, _ := newRepo(t)

				return repo
			},
			want: 0,
		},
		{
			name: "two records returns both",
			setup: func(t *testing.T) *webhookrepo.Repository {
				t.Helper()
				repo, _ := newRepo(t)

				require.NoError(t, repo.Create(t.Context(), newTestWebhook("https://a.example.com")))
				require.NoError(t, repo.Create(t.Context(), newTestWebhook("https://b.example.com")))

				return repo
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := tt.setup(t)

			got, err := repo.List(t.Context())
			require.NoError(t, err)
			assert.Len(t, got, tt.want)
		})
	}
}

func TestRepository_WithTx_RollbackDiscardsWrites(t *testing.T) {
	t.Parallel()

	repo, mgr := newRepo(t)
	ctx := t.Context()

	w1 := newTestWebhook("https://example.com/hook1")
	w2 := newTestWebhook("https://example.com/hook2")

	err := mgr.WithTx(ctx, func(ctx context.Context) error {
		if err := repo.Create(ctx, w1); err != nil {
			return err
		}
		if err := repo.Create(ctx, w2); err != nil {
			return err
		}

		return assert.AnError
	})
	require.ErrorIs(t, err, assert.AnError)

	got, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, got)
}
