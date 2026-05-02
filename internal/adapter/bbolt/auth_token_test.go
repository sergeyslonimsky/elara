package bbolt_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bboltadapter "github.com/sergeyslonimsky/elara/internal/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

func newTestPAT(id, email, hash string) *domain.PAT {
	return &domain.PAT{
		ID:        id,
		UserEmail: email,
		Name:      "Test Token " + id,
		TokenHash: hash,
		CreatedAt: time.Now(),
	}
}

func TestPATRepo_Create(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPATRepo(store)
	ctx := t.Context()

	pat := newTestPAT("pat-1", "alice@example.com", "hash-abc123")
	err := repo.Create(ctx, pat)
	require.NoError(t, err)
}

func TestPATRepo_GetByHash(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPATRepo(store)
	ctx := t.Context()

	pat := newTestPAT("pat-2", "bob@example.com", "hash-def456")
	require.NoError(t, repo.Create(ctx, pat))

	got, err := repo.GetByHash(ctx, "hash-def456")
	require.NoError(t, err)
	assert.Equal(t, "pat-2", got.ID)
	assert.Equal(t, "bob@example.com", got.UserEmail)
	assert.Equal(t, "hash-def456", got.TokenHash)
}

func TestPATRepo_GetByHash_Missing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPATRepo(store)
	ctx := t.Context()

	_, err := repo.GetByHash(ctx, "nonexistent-hash")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestPATRepo_List_ByUser(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPATRepo(store)
	ctx := t.Context()

	require.NoError(t, repo.Create(ctx, newTestPAT("pat-u1", "carol@example.com", "hash-u1")))
	require.NoError(t, repo.Create(ctx, newTestPAT("pat-u2", "carol@example.com", "hash-u2")))
	require.NoError(t, repo.Create(ctx, newTestPAT("pat-u3", "dave@example.com", "hash-u3")))

	carolTokens, err := repo.List(ctx, "carol@example.com")
	require.NoError(t, err)
	assert.Len(t, carolTokens, 2)

	daveTokens, err := repo.List(ctx, "dave@example.com")
	require.NoError(t, err)
	assert.Len(t, daveTokens, 1)
}

func TestPATRepo_List_All(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPATRepo(store)
	ctx := t.Context()

	// Empty list.
	all, err := repo.List(ctx, "")
	require.NoError(t, err)
	assert.Empty(t, all)

	// Populate with tokens for different users.
	require.NoError(t, repo.Create(ctx, newTestPAT("pat-a1", "eve@example.com", "hash-a1")))
	require.NoError(t, repo.Create(ctx, newTestPAT("pat-a2", "frank@example.com", "hash-a2")))
	require.NoError(t, repo.Create(ctx, newTestPAT("pat-a3", "grace@example.com", "hash-a3")))

	all, err = repo.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestPATRepo_Delete_ByID(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPATRepo(store)
	ctx := t.Context()

	pat := newTestPAT("pat-del1", "henry@example.com", "hash-del1")
	require.NoError(t, repo.Create(ctx, pat))

	require.NoError(t, repo.Delete(ctx, "pat-del1"))

	_, err := repo.GetByHash(ctx, "hash-del1")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestPATRepo_Delete_Missing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPATRepo(store)
	ctx := t.Context()

	err := repo.Delete(ctx, "nonexistent-id")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestPATRepo_GetByID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T, repo *bboltadapter.PATRepo)
		id      string
		wantErr error
		verify  func(t *testing.T, got *domain.PAT)
	}{
		{
			name: "happy path returns correct PAT",
			setup: func(t *testing.T, repo *bboltadapter.PATRepo) {
				t.Helper()

				pat := newTestPAT("pat-id-1", "getbyid@example.com", "hash-getbyid-1")
				require.NoError(t, repo.Create(t.Context(), pat))
			},
			id: "pat-id-1",
			verify: func(t *testing.T, got *domain.PAT) {
				t.Helper()

				assert.Equal(t, "pat-id-1", got.ID)
				assert.Equal(t, "getbyid@example.com", got.UserEmail)
				assert.Equal(t, "hash-getbyid-1", got.TokenHash)
			},
		},
		{
			name:    "not found returns ErrNotFound",
			setup:   func(_ *testing.T, _ *bboltadapter.PATRepo) {},
			id:      "nonexistent-id",
			wantErr: domain.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := newTestStore(t)
			repo := bboltadapter.NewPATRepo(store)

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

func TestPATRepo_Delete_RemovesSecondaryIndex(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPATRepo(store)
	ctx := t.Context()

	pat := newTestPAT("pat-sidx-del", "sidx@example.com", "hash-sidx-del")
	require.NoError(t, repo.Create(ctx, pat))

	// Confirm both lookups work before deletion.
	_, err := repo.GetByID(ctx, "pat-sidx-del")
	require.NoError(t, err)

	_, err = repo.GetByHash(ctx, "hash-sidx-del")
	require.NoError(t, err)

	// Delete via ID.
	require.NoError(t, repo.Delete(ctx, "pat-sidx-del"))

	// Secondary index must be gone.
	_, err = repo.GetByID(ctx, "pat-sidx-del")
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrNotFound)

	// Primary key must also be gone.
	_, err = repo.GetByHash(ctx, "hash-sidx-del")
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestPATRepo_UpdateLastUsed(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPATRepo(store)
	ctx := t.Context()

	pat := newTestPAT("pat-upd1", "ivan@example.com", "hash-upd1")
	require.NoError(t, repo.Create(ctx, pat))

	usedAt := time.Now().Add(time.Minute)
	require.NoError(t, repo.UpdateLastUsed(ctx, "hash-upd1", "192.168.1.1", usedAt))

	got, err := repo.GetByHash(ctx, "hash-upd1")
	require.NoError(t, err)
	require.NotNil(t, got.LastUsedAt)
	assert.Equal(t, usedAt.Unix(), got.LastUsedAt.Unix())
	assert.Equal(t, "192.168.1.1", got.LastUsedIP)
}
