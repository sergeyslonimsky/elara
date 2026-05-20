package bbolt_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	bboltadapter "github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
)

func TestUserRepo_Upsert_New(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(store)
	ctx := t.Context()

	user := &domain.User{
		Email:       "alice@example.com",
		Name:        "Alice",
		Picture:     "https://example.com/alice.png",
		Provider:    "oidc",
		LastLoginAt: time.Now(),
	}

	err := repo.Upsert(ctx, user)
	require.NoError(t, err)
	assert.False(t, user.CreatedAt.IsZero(), "CreatedAt should be set after first upsert")

	got, err := repo.Get(ctx, user.Email)
	require.NoError(t, err)
	assert.Equal(t, user.Email, got.Email)
	assert.Equal(t, user.Name, got.Name)
	assert.Equal(t, user.Provider, got.Provider)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestUserRepo_Upsert_Existing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(store)
	ctx := t.Context()

	user := &domain.User{
		Email:       "bob@example.com",
		Name:        "Bob",
		Provider:    "oidc",
		LastLoginAt: time.Now(),
	}
	require.NoError(t, repo.Upsert(ctx, user))

	originalCreatedAt := user.CreatedAt

	// Update name and last login.
	user.Name = "Bob Updated"
	user.LastLoginAt = time.Now().Add(time.Hour)
	require.NoError(t, repo.Upsert(ctx, user))

	got, err := repo.Get(ctx, user.Email)
	require.NoError(t, err)
	assert.Equal(t, "Bob Updated", got.Name)
	assert.Equal(t, originalCreatedAt.UnixNano(), got.CreatedAt.UnixNano(), "CreatedAt must not change on update")
}

func TestUserRepo_Get_Existing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(store)
	ctx := t.Context()

	user := &domain.User{Email: "carol@example.com", Name: "Carol", Provider: "oidc", LastLoginAt: time.Now()}
	require.NoError(t, repo.Upsert(ctx, user))

	got, err := repo.Get(ctx, "carol@example.com")
	require.NoError(t, err)
	assert.Equal(t, "carol@example.com", got.Email)
	assert.Equal(t, "Carol", got.Name)
}

func TestUserRepo_Get_Missing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(store)
	ctx := t.Context()

	_, err := repo.Get(ctx, "nobody@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestUserRepo_ListAll_Empty(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(store)
	ctx := t.Context()

	users, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, users)
}

func TestUserRepo_ListAll_Multiple(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(store)
	ctx := t.Context()

	emails := []string{"dave@example.com", "eve@example.com", "frank@example.com"}
	for _, email := range emails {
		u := &domain.User{Email: email, Name: email, Provider: "oidc", LastLoginAt: time.Now()}
		require.NoError(t, repo.Upsert(ctx, u))
	}

	users, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Len(t, users, len(emails))
}

func TestUserRepo_List(t *testing.T) {
	t.Parallel()

	// Seeds used across cases. Stable email/name pairs let us assert sort
	// ordering exactly. LastLoginAt is staggered so "last_login" sort is
	// deterministic if added in future.
	seed := []*domain.User{
		{Email: "alice@example.com", Name: "Alice", Provider: "basic-auth"},
		{Email: "bob@example.com", Name: "Bob", Provider: "basic-auth"},
		{Email: "carol@example.com", Name: "Carol", Provider: "basic-auth"},
		{Email: "dave@example.com", Name: "Dave", Provider: "basic-auth"},
	}

	tests := []struct {
		name       string
		filter     domain.UserFilter
		params     domain.UserListParams
		wantEmails []string // expected emails in result order; ElementsMatch when order is not asserted
		wantTotal  int
		ordered    bool // when true, assert exact result order
	}{
		{
			name:       "AnyUser returns all sorted by email asc by default",
			filter:     domain.UserFilter{AnyUser: true},
			params:     domain.UserListParams{},
			wantEmails: []string{"alice@example.com", "bob@example.com", "carol@example.com", "dave@example.com"},
			wantTotal:  4,
			ordered:    true,
		},
		{
			name: "explicit usernames filter returns subset",
			filter: domain.UserFilter{Usernames: map[string]struct{}{
				"alice@example.com": {},
				"carol@example.com": {},
			}},
			wantEmails: []string{"alice@example.com", "carol@example.com"},
			wantTotal:  2,
		},
		{
			name: "explicit with missing username silently skips",
			filter: domain.UserFilter{Usernames: map[string]struct{}{
				"alice@example.com":   {},
				"missing@example.com": {},
			}},
			wantEmails: []string{"alice@example.com"},
			wantTotal:  1,
		},
		{
			name:       "search filters case-insensitive substring on email/name",
			filter:     domain.UserFilter{AnyUser: true, Search: "ALI"},
			wantEmails: []string{"alice@example.com"},
			wantTotal:  1,
		},
		{
			name: "search combined with explicit filter intersects",
			filter: domain.UserFilter{
				Usernames: map[string]struct{}{
					"alice@example.com": {},
					"bob@example.com":   {},
				},
				Search: "bob",
			},
			wantEmails: []string{"bob@example.com"},
			wantTotal:  1,
		},
		{
			name:       "pagination offset+limit slices, total preserved",
			filter:     domain.UserFilter{AnyUser: true},
			params:     domain.UserListParams{Offset: 1, Limit: 2},
			wantEmails: []string{"bob@example.com", "carol@example.com"},
			wantTotal:  4,
			ordered:    true,
		},
		{
			name:       "sort by name desc",
			filter:     domain.UserFilter{AnyUser: true},
			params:     domain.UserListParams{Sort: domain.SortParams{Field: "name", Desc: true}},
			wantEmails: []string{"dave@example.com", "carol@example.com", "bob@example.com", "alice@example.com"},
			wantTotal:  4,
			ordered:    true,
		},
		{
			name:       "empty filter (AnyUser:false, Usernames:nil) returns empty list",
			filter:     domain.UserFilter{},
			wantEmails: nil,
			wantTotal:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := newTestStore(t)
			repo := bboltadapter.NewUserRepo(store)
			ctx := t.Context()

			for _, u := range seed {
				cp := *u
				cp.LastLoginAt = time.Now()
				require.NoError(t, repo.Upsert(ctx, &cp))
			}

			got, total, err := repo.List(ctx, tt.filter, tt.params)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTotal, total)

			emails := make([]string, 0, len(got))
			for _, u := range got {
				emails = append(emails, u.Email)
			}

			if tt.ordered {
				assert.Equal(t, tt.wantEmails, emails)
			} else {
				assert.ElementsMatch(t, tt.wantEmails, emails)
			}
		})
	}
}

func TestUserRepo_SetPassword(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(store)
	ctx := t.Context()

	email := "user@example.com"
	user := &domain.User{Email: email, Name: "User", Provider: "basic"}
	require.NoError(t, repo.Upsert(ctx, user))

	hash := "some-hash"
	err := repo.SetPassword(ctx, email, hash, true)
	require.NoError(t, err)

	got, err := repo.Get(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, hash, got.PasswordHash)
	assert.True(t, got.PasswordChangeRequired)

	// Update without change required.
	err = repo.SetPassword(ctx, email, "new-hash", false)
	require.NoError(t, err)

	got, err = repo.Get(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, "new-hash", got.PasswordHash)
	assert.False(t, got.PasswordChangeRequired)
}

func TestUserRepo_SystemAndSource(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(store)
	ctx := t.Context()

	user := &domain.User{
		Email:    "sys@example.com",
		Name:     "System User",
		Provider: domain.ProviderBasicAuth,
		System:   true,
		Source:   "seed",
	}

	// Upsert (New)
	require.NoError(t, repo.Upsert(ctx, user))

	// Get and verify
	got, err := repo.Get(ctx, user.Email)
	require.NoError(t, err)
	assert.True(t, got.System)
	assert.Equal(t, "seed", got.Source)

	// Upsert (Existing) - should preserve System flag
	user.Name = "Updated Name"
	user.System = false
	user.Source = "manual"
	require.NoError(t, repo.Upsert(ctx, user))

	final, err := repo.Get(ctx, user.Email)
	require.NoError(t, err)
	assert.True(t, final.System, "System flag must be preserved from existing record")
	assert.Equal(t, "manual", final.Source, "Source should be updated if provided")

	// Upsert (Existing) - preserve Source
	user.Source = ""
	require.NoError(t, repo.Upsert(ctx, user))
	final2, err := repo.Get(ctx, user.Email)
	require.NoError(t, err)
	assert.Equal(t, "manual", final2.Source, "Source should be preserved if not provided")
}
