package bbolt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bboltadapter "github.com/sergeyslonimsky/elara/internal/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

func newTestWebhook(url string) *domain.Webhook {
	return &domain.Webhook{
		URL:     url,
		Events:  []domain.WebhookEventType{domain.WebhookEventCreated, domain.WebhookEventUpdated},
		Enabled: true,
	}
}

func TestWebhookRepo_Create(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewWebhookRepo(store)
	ctx := t.Context()

	w := newTestWebhook("https://example.com/hook")

	err := repo.Create(ctx, w)
	require.NoError(t, err)
	assert.NotEmpty(t, w.ID)
	assert.False(t, w.CreatedAt.IsZero())
	assert.False(t, w.UpdatedAt.IsZero())
}

func TestWebhookRepo_Get(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewWebhookRepo(store)
	ctx := t.Context()

	w := newTestWebhook("https://example.com/hook")
	require.NoError(t, repo.Create(ctx, w))

	got, err := repo.Get(ctx, w.ID)
	require.NoError(t, err)
	assert.Equal(t, w.ID, got.ID)
	assert.Equal(t, w.URL, got.URL)
	assert.Equal(t, w.Events, got.Events)
	assert.True(t, got.Enabled)
}

func TestWebhookRepo_Get_NotFound(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewWebhookRepo(store)
	ctx := t.Context()

	_, err := repo.Get(ctx, "nonexistent-id")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestWebhookRepo_List(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewWebhookRepo(store)
	ctx := t.Context()

	list, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)

	w1 := newTestWebhook("https://example.com/hook1")
	w2 := newTestWebhook("https://example.com/hook2")
	require.NoError(t, repo.Create(ctx, w1))
	require.NoError(t, repo.Create(ctx, w2))

	list, err = repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestWebhookRepo_Update(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewWebhookRepo(store)
	ctx := t.Context()

	w := newTestWebhook("https://example.com/hook")
	require.NoError(t, repo.Create(ctx, w))

	w.URL = "https://updated.example.com/hook"
	w.Enabled = false

	require.NoError(t, repo.Update(ctx, w))

	got, err := repo.Get(ctx, w.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://updated.example.com/hook", got.URL)
	assert.False(t, got.Enabled)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestWebhookRepo_Delete(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewWebhookRepo(store)
	ctx := t.Context()

	w := newTestWebhook("https://example.com/hook")
	require.NoError(t, repo.Create(ctx, w))

	require.NoError(t, repo.Delete(ctx, w.ID))

	_, err := repo.Get(ctx, w.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestWebhookRepo_Create_DuplicateID(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewWebhookRepo(store)
	ctx := t.Context()

	w := newTestWebhook("https://example.com/hook")
	require.NoError(t, repo.Create(ctx, w))

	w2 := &domain.Webhook{
		ID:      w.ID,
		URL:     "https://other.example.com/hook",
		Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
		Enabled: true,
	}

	err := repo.Create(ctx, w2)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
}

func TestWebhookRepo_Update_NotFound(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewWebhookRepo(store)
	ctx := t.Context()

	w := &domain.Webhook{
		ID:      "nonexistent-id",
		URL:     "https://example.com/hook",
		Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
		Enabled: true,
	}

	err := repo.Update(ctx, w)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestWebhookRepo_Delete_NotFound(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewWebhookRepo(store)
	ctx := t.Context()

	err := repo.Delete(ctx, "nonexistent-id")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestWebhookRepo_Update_PreservesSecretWhenEmpty(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewWebhookRepo(store)
	ctx := t.Context()

	w := newTestWebhook("https://example.com/hook")
	w.Secret = "original-secret"
	require.NoError(t, repo.Create(ctx, w))

	w.Secret = ""
	require.NoError(t, repo.Update(ctx, w))

	got, err := repo.Get(ctx, w.ID)
	require.NoError(t, err)
	assert.Equal(t, "original-secret", got.Secret)
}
