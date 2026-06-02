package bbolt_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	bboltadapter "github.com/sergeyslonimsky/elara/internal/storage/bbolt"
)

func TestUserRepo_Create(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	user := &domain.User{
		ID:          uuid.New(),
		Email:       "alice@example.com",
		DisplayName: "Alice",
		Picture:     "https://example.com/alice.png",
		Identities:  []domain.Identity{{Provider: "oidc", Subject: "alice@example.com"}},
		LastLoginAt: time.Now(),
	}

	err := repo.Create(ctx, user)
	require.NoError(t, err)
	assert.False(t, user.CreatedAt.IsZero(), "CreatedAt should be set after Create")

	got, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.Email, got.Email)
	assert.Equal(t, user.DisplayName, got.DisplayName)
	assert.Equal(t, user.Identities, got.Identities)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestUserRepo_Update_Existing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	user := &domain.User{
		ID:          uuid.New(),
		Email:       "bob@example.com",
		DisplayName: "Bob",
		Identities:  []domain.Identity{{Provider: "oidc", Subject: "bob@example.com"}},
		LastLoginAt: time.Now(),
	}
	require.NoError(t, repo.Create(ctx, user))

	// Read-modify-write: load existing then mutate. Update is a full
	// overwrite — the caller is responsible for carrying CreatedAt through.
	loaded, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	originalCreatedAt := loaded.CreatedAt

	loaded.DisplayName = "Bob Updated"
	loaded.LastLoginAt = time.Now().Add(time.Hour)
	require.NoError(t, repo.Update(ctx, loaded))

	got, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "Bob Updated", got.DisplayName)
	assert.Equal(
		t,
		originalCreatedAt.UnixNano(),
		got.CreatedAt.UnixNano(),
		"CreatedAt must survive read-modify-write",
	)
}

func TestUserRepo_GetByIdentity(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	user := &domain.User{
		ID:          uuid.New(),
		Email:       "carol@example.com",
		DisplayName: "Carol",
		Identities:  []domain.Identity{{Provider: "oidc", Subject: "carol-subject"}},
		LastLoginAt: time.Now(),
	}
	require.NoError(t, repo.Create(ctx, user))

	got, err := repo.GetByIdentity(ctx, "oidc", "carol-subject")
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
	assert.Equal(t, "Carol", got.DisplayName)
}

func TestUserRepo_GetByIdentity_Missing(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	_, err := repo.GetByIdentity(ctx, "oidc", "nobody")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestUserRepo_ListAll_Empty(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	users, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, users)
}

func TestUserRepo_ListAll_Multiple(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	emails := []string{"dave@example.com", "eve@example.com", "frank@example.com"}
	for _, email := range emails {
		u := &domain.User{
			ID:          uuid.New(),
			Email:       email,
			DisplayName: email,
			Identities:  []domain.Identity{{Provider: domain.ProviderOIDC, Subject: email}},
			LastLoginAt: time.Now(),
		}
		require.NoError(t, repo.Create(ctx, u))
	}

	users, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Len(t, users, len(emails))
}

func TestUserRepo_List(t *testing.T) {
	t.Parallel()

	// seedUsers upserts the canonical four users and returns them with their
	// auto-assigned IDs populated. Each sub-test creates its own store.
	seedUsers := func(t *testing.T, repo *bboltadapter.UserRepo) []*domain.User {
		t.Helper()

		users := []*domain.User{
			{
				ID:          uuid.New(),
				Email:       "alice@example.com",
				DisplayName: "Alice",
				Identities:  []domain.Identity{{Provider: domain.ProviderBasic, Subject: "alice@example.com"}},
			},
			{
				ID:          uuid.New(),
				Email:       "bob@example.com",
				DisplayName: "Bob",
				Identities:  []domain.Identity{{Provider: domain.ProviderBasic, Subject: "bob@example.com"}},
			},
			{
				ID:          uuid.New(),
				Email:       "carol@example.com",
				DisplayName: "Carol",
				Identities:  []domain.Identity{{Provider: domain.ProviderBasic, Subject: "carol@example.com"}},
			},
			{
				ID:          uuid.New(),
				Email:       "dave@example.com",
				DisplayName: "Dave",
				Identities:  []domain.Identity{{Provider: domain.ProviderBasic, Subject: "dave@example.com"}},
			},
		}
		for _, u := range users {
			u.LastLoginAt = time.Now()
			require.NoError(t, repo.Create(t.Context(), u))
		}

		return users
	}

	type buildFilterFunc func(
		t *testing.T,
		repo *bboltadapter.UserRepo,
	) (domain.UserFilter, domain.UserListParams, []string, int, bool)

	tests := []struct {
		name      string
		buildFunc buildFilterFunc
	}{
		{
			name: "AnyUser returns all sorted by email asc by default",
			buildFunc: func(
				t *testing.T,
				repo *bboltadapter.UserRepo,
			) (domain.UserFilter, domain.UserListParams, []string, int, bool) {
				t.Helper()
				seedUsers(t, repo)

				return domain.UserFilter{AnyUser: true}, domain.UserListParams{},
					[]string{"alice@example.com", "bob@example.com", "carol@example.com", "dave@example.com"}, 4, true
			},
		},
		{
			name: "explicit UserIDs filter returns subset",
			buildFunc: func(
				t *testing.T,
				repo *bboltadapter.UserRepo,
			) (domain.UserFilter, domain.UserListParams, []string, int, bool) {
				t.Helper()
				seeded := seedUsers(t, repo)
				// alice and carol by ID.
				ids := map[string]struct{}{seeded[0].ID.String(): {}, seeded[2].ID.String(): {}}

				return domain.UserFilter{UserIDs: ids}, domain.UserListParams{},
					[]string{"alice@example.com", "carol@example.com"}, 2, false
			},
		},
		{
			name: "explicit with missing UserID silently skips",
			buildFunc: func(
				t *testing.T,
				repo *bboltadapter.UserRepo,
			) (domain.UserFilter, domain.UserListParams, []string, int, bool) {
				t.Helper()
				seeded := seedUsers(t, repo)
				ids := map[string]struct{}{seeded[0].ID.String(): {}, "missing-uuid": {}}

				return domain.UserFilter{UserIDs: ids}, domain.UserListParams{},
					[]string{"alice@example.com"}, 1, false
			},
		},
		{
			name: "search filters case-insensitive substring on email/name",
			buildFunc: func(
				t *testing.T,
				repo *bboltadapter.UserRepo,
			) (domain.UserFilter, domain.UserListParams, []string, int, bool) {
				t.Helper()
				seedUsers(t, repo)

				return domain.UserFilter{AnyUser: true, Search: "ALI"}, domain.UserListParams{},
					[]string{"alice@example.com"}, 1, false
			},
		},
		{
			name: "search combined with explicit UserIDs filter intersects",
			buildFunc: func(
				t *testing.T,
				repo *bboltadapter.UserRepo,
			) (domain.UserFilter, domain.UserListParams, []string, int, bool) {
				t.Helper()
				seeded := seedUsers(t, repo)
				ids := map[string]struct{}{seeded[0].ID.String(): {}, seeded[1].ID.String(): {}} // alice + bob

				return domain.UserFilter{UserIDs: ids, Search: "bob"}, domain.UserListParams{},
					[]string{"bob@example.com"}, 1, false
			},
		},
		{
			name: "pagination offset+limit slices, total preserved",
			buildFunc: func(
				t *testing.T,
				repo *bboltadapter.UserRepo,
			) (domain.UserFilter, domain.UserListParams, []string, int, bool) {
				t.Helper()
				seedUsers(t, repo)

				return domain.UserFilter{AnyUser: true}, domain.UserListParams{Offset: 1, Limit: 2},
					[]string{"bob@example.com", "carol@example.com"}, 4, true
			},
		},
		{
			name: "sort by name desc",
			buildFunc: func(
				t *testing.T,
				repo *bboltadapter.UserRepo,
			) (domain.UserFilter, domain.UserListParams, []string, int, bool) {
				t.Helper()
				seedUsers(t, repo)

				return domain.UserFilter{AnyUser: true},
					domain.UserListParams{Sort: domain.SortParams{Field: "name", Desc: true}},
					[]string{"dave@example.com", "carol@example.com", "bob@example.com", "alice@example.com"}, 4, true
			},
		},
		{
			name: "empty filter (AnyUser:false, UserIDs:nil) returns empty list",
			buildFunc: func(
				t *testing.T,
				repo *bboltadapter.UserRepo,
			) (domain.UserFilter, domain.UserListParams, []string, int, bool) {
				t.Helper()
				seedUsers(t, repo)

				return domain.UserFilter{}, domain.UserListParams{}, nil, 0, false
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := newTestStore(t)
			repo := bboltadapter.NewUserRepo(bboltadapter.NewManager(store.DB()))

			filter, params, wantEmails, wantTotal, ordered := tt.buildFunc(t, repo)

			got, total, err := repo.List(t.Context(), filter, params)
			require.NoError(t, err)
			assert.Equal(t, wantTotal, total)

			emails := make([]string, 0, len(got))
			for _, u := range got {
				emails = append(emails, u.Email)
			}

			if ordered {
				assert.Equal(t, wantEmails, emails)
			} else {
				assert.ElementsMatch(t, wantEmails, emails)
			}
		})
	}
}

func TestUserRepo_SetPassword(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	email := "user@example.com"
	user := &domain.User{
		ID:          uuid.New(),
		Email:       email,
		DisplayName: "User",
		Identities:  []domain.Identity{{Provider: "basic", Subject: email}},
	}
	require.NoError(t, repo.Create(ctx, user))

	hash := "some-hash"
	err := repo.SetPassword(ctx, user.ID, hash, true)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, hash, got.PasswordHash)
	assert.True(t, got.PasswordChangeRequired)

	// Update without change required.
	err = repo.SetPassword(ctx, user.ID, "new-hash", false)
	require.NoError(t, err)

	got, err = repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "new-hash", got.PasswordHash)
	assert.False(t, got.PasswordChangeRequired)
}

// TestUserRepo_SystemFlagRoundTrip verifies that System and Source are
// persisted and surfaced verbatim by the repo. The repo no longer merges or
// "preserves" these fields across Update — that policy moved to UserService
// (which rejects identity removal for non-system users). Callers must
// read-modify-write to keep fields they don't intend to change.
func TestUserRepo_SystemFlagRoundTrip(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewUserRepo(bboltadapter.NewManager(store.DB()))
	ctx := t.Context()

	user := &domain.User{
		ID:          uuid.New(),
		Email:       "sys@example.com",
		DisplayName: "System User",
		Identities:  []domain.Identity{{Provider: "system", Subject: "sys@example.com"}},
		System:      true,
	}

	require.NoError(t, repo.Create(ctx, user))

	got, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.True(t, got.System)

	// Read-modify-write: name only; System survives because we
	// loaded it off disk and pass it through.
	got.DisplayName = "Updated Name"
	require.NoError(t, repo.Update(ctx, got))

	final, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.True(t, final.System)
	assert.Equal(t, "Updated Name", final.DisplayName)
}
