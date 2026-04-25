package bbolt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bboltadapter "github.com/sergeyslonimsky/elara/internal/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

const testJSONSchema = `{"type": "object", "properties": {"host": {"type": "string"}}}`

func TestSchemaRepo_Attach_Get(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewSchemaRepo(store)
	ctx := t.Context()

	s := &domain.SchemaAttachment{
		ID:          "schema-1",
		Namespace:   "production",
		PathPattern: "/services/**",
		JSONSchema:  testJSONSchema,
	}

	err := repo.Attach(ctx, s)
	require.NoError(t, err)
	assert.False(t, s.CreatedAt.IsZero())

	got, err := repo.Get(ctx, "production", "/services/**")
	require.NoError(t, err)
	assert.Equal(t, "schema-1", got.ID)
	assert.Equal(t, "production", got.Namespace)
	assert.Equal(t, "/services/**", got.PathPattern)
	assert.JSONEq(t, testJSONSchema, got.JSONSchema)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestSchemaRepo_Attach_Upsert(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewSchemaRepo(store)
	ctx := t.Context()

	s := &domain.SchemaAttachment{
		ID:          "schema-1",
		Namespace:   "production",
		PathPattern: "/services/**",
		JSONSchema:  testJSONSchema,
	}

	require.NoError(t, repo.Attach(ctx, s))
	originalCreatedAt := s.CreatedAt

	const updatedSchema = `{"type": "object"}`
	s2 := &domain.SchemaAttachment{
		ID:          "schema-2",
		Namespace:   "production",
		PathPattern: "/services/**",
		JSONSchema:  updatedSchema,
	}

	require.NoError(t, repo.Attach(ctx, s2))

	got, err := repo.Get(ctx, "production", "/services/**")
	require.NoError(t, err)
	assert.Equal(t, "schema-2", got.ID)
	assert.JSONEq(t, updatedSchema, got.JSONSchema)
	assert.True(t, originalCreatedAt.Equal(got.CreatedAt), "CreatedAt should be preserved on update")
}

func TestSchemaRepo_Detach(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewSchemaRepo(store)
	ctx := t.Context()

	s := &domain.SchemaAttachment{
		ID:          "schema-1",
		Namespace:   "production",
		PathPattern: "/services/**",
		JSONSchema:  testJSONSchema,
	}

	require.NoError(t, repo.Attach(ctx, s))

	err := repo.Detach(ctx, "production", "/services/**")
	require.NoError(t, err)

	_, err = repo.Get(ctx, "production", "/services/**")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestSchemaRepo_Detach_NotFound(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewSchemaRepo(store)
	ctx := t.Context()

	err := repo.Detach(ctx, "production", "/nonexistent/**")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestSchemaRepo_List(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewSchemaRepo(store)
	ctx := t.Context()

	schemas := []*domain.SchemaAttachment{
		{ID: "s1", Namespace: "ns1", PathPattern: "/a/**", JSONSchema: testJSONSchema},
		{ID: "s2", Namespace: "ns1", PathPattern: "/b/**", JSONSchema: testJSONSchema},
		{ID: "s3", Namespace: "ns2", PathPattern: "/c/**", JSONSchema: testJSONSchema},
	}

	for _, s := range schemas {
		require.NoError(t, repo.Attach(ctx, s))
	}

	list, err := repo.List(ctx, "ns1")
	require.NoError(t, err)
	assert.Len(t, list, 2)

	for _, item := range list {
		assert.Equal(t, "ns1", item.Namespace)
	}
}

func TestSchemaRepo_List_Empty(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	repo := bboltadapter.NewSchemaRepo(store)
	ctx := t.Context()

	list, err := repo.List(ctx, "empty-namespace")
	require.NoError(t, err)
	assert.Empty(t, list)
}
