package user_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	userrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/user"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

func newRepo(t *testing.T) *userrepo.Repository {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	mgr := pkgbbolt.NewManager(store.DB())

	return userrepo.NewRepository(mgr)
}

func newTestUser(t *testing.T, email string) *domain.User {
	t.Helper()

	return &domain.User{
		ID:          uuid.New(),
		Email:       email,
		DisplayName: "Display " + email,
		Status:      domain.UserStatusActive,
		Identities: []domain.Identity{
			{Provider: domain.ProviderBasic, Subject: email},
		},
	}
}

func TestRepository_CreateAndGetByID(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	ctx := t.Context()

	u := newTestUser(t, "alice@example.com")
	u.System = true
	u.MembershipVersion = 7

	require.NoError(t, repo.Create(ctx, u))
	assert.False(t, u.CreatedAt.IsZero(), "CreatedAt should be auto-set")

	got, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, u.ID, got.ID)
	assert.Equal(t, u.Email, got.Email)
	assert.Equal(t, u.DisplayName, got.DisplayName)
	assert.Equal(t, u.Status, got.Status)
	assert.True(t, got.System)
	assert.Equal(t, int64(7), got.MembershipVersion)
	assert.Equal(t, u.Identities, got.Identities)
}

func TestRepository_Create_PreservesProvidedCreatedAt(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	ctx := t.Context()

	when := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	u := newTestUser(t, "ts@example.com")
	u.CreatedAt = when

	require.NoError(t, repo.Create(ctx, u))

	got, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	assert.True(t, got.CreatedAt.Equal(when))
}

func TestRepository_Create_DuplicateID(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	ctx := t.Context()

	u := newTestUser(t, "dup@example.com")
	require.NoError(t, repo.Create(ctx, u))

	dup := newTestUser(t, "other@example.com")
	dup.ID = u.ID

	err := repo.Create(ctx, dup)
	require.ErrorIs(t, err, storage.ErrResourceAlreadyExists)
}

func TestRepository_Create_EmailTaken(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	ctx := t.Context()

	u1 := newTestUser(t, "same@example.com")
	require.NoError(t, repo.Create(ctx, u1))

	u2 := newTestUser(t, "same@example.com")
	// Different identity so we hit the email collision, not identity collision.
	u2.Identities = []domain.Identity{{Provider: domain.ProviderBasic, Subject: "different"}}

	err := repo.Create(ctx, u2)
	require.ErrorIs(t, err, domain.ErrEmailTaken)
}

func TestRepository_Create_IdentityTaken(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	ctx := t.Context()

	u1 := newTestUser(t, "a@example.com")
	require.NoError(t, repo.Create(ctx, u1))

	u2 := newTestUser(t, "b@example.com")
	u2.Identities = u1.Identities // collide on identity

	err := repo.Create(ctx, u2)
	require.ErrorIs(t, err, domain.ErrIdentityTaken)
}

func TestRepository_Create_EmptyEmail_NoIndexEntry(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	ctx := t.Context()

	u1 := newTestUser(t, "")
	u1.Identities = []domain.Identity{{Provider: domain.ProviderBasic, Subject: "noemail1"}}
	require.NoError(t, repo.Create(ctx, u1))

	// Second user with empty email must not collide on email index.
	u2 := newTestUser(t, "")
	u2.Identities = []domain.Identity{{Provider: domain.ProviderBasic, Subject: "noemail2"}}
	require.NoError(t, repo.Create(ctx, u2))

	// Empty email lookup is not found.
	_, err := repo.GetByEmail(ctx, "")
	require.ErrorIs(t, err, storage.ErrResourceNotFound)
}

func TestRepository_Update(t *testing.T) {
	t.Parallel()

	t.Run("happy path persists DisplayName", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		ctx := t.Context()

		u := newTestUser(t, "u@example.com")
		require.NoError(t, repo.Create(ctx, u))

		u.DisplayName = "Changed"
		require.NoError(t, repo.Update(ctx, u))

		got, err := repo.GetByID(ctx, u.ID)
		require.NoError(t, err)
		assert.Equal(t, "Changed", got.DisplayName)
	})

	t.Run("missing returns ErrResourceNotFound", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		err := repo.Update(t.Context(), newTestUser(t, "ghost@example.com"))
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_Update_ReconcilesIdentityIndex(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	ctx := t.Context()

	identA := domain.Identity{Provider: domain.ProviderBasic, Subject: "subA"}
	identB := domain.Identity{Provider: domain.ProviderOIDC, Subject: "subB"}

	u := newTestUser(t, "ident@example.com")
	u.Identities = []domain.Identity{identA}
	require.NoError(t, repo.Create(ctx, u))

	// Add B (kept A).
	u.Identities = []domain.Identity{identA, identB}
	require.NoError(t, repo.Update(ctx, u))

	gotA, err := repo.GetByIdentity(ctx, string(identA.Provider), identA.Subject)
	require.NoError(t, err)
	assert.Equal(t, u.ID, gotA.ID)

	gotB, err := repo.GetByIdentity(ctx, string(identB.Provider), identB.Subject)
	require.NoError(t, err)
	assert.Equal(t, u.ID, gotB.ID)

	// Remove A (only B remains).
	u.Identities = []domain.Identity{identB}
	require.NoError(t, repo.Update(ctx, u))

	_, err = repo.GetByIdentity(ctx, string(identA.Provider), identA.Subject)
	require.ErrorIs(t, err, storage.ErrResourceNotFound)

	gotB2, err := repo.GetByIdentity(ctx, string(identB.Provider), identB.Subject)
	require.NoError(t, err)
	assert.Equal(t, u.ID, gotB2.ID)
}

func TestRepository_Update_ReconcilesEmailIndex(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	ctx := t.Context()

	u := newTestUser(t, "a@x.com")
	require.NoError(t, repo.Create(ctx, u))

	u.Email = "b@x.com"
	require.NoError(t, repo.Update(ctx, u))

	_, err := repo.GetByEmail(ctx, "a@x.com")
	require.ErrorIs(t, err, storage.ErrResourceNotFound)

	got, err := repo.GetByEmail(ctx, "b@x.com")
	require.NoError(t, err)
	assert.Equal(t, u.ID, got.ID)
}

func TestRepository_Update_IdentityTakenByOtherUser(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	ctx := t.Context()

	u1 := newTestUser(t, "u1@example.com")
	require.NoError(t, repo.Create(ctx, u1))

	u2 := newTestUser(t, "u2@example.com")
	u2.Identities = []domain.Identity{{Provider: domain.ProviderBasic, Subject: "u2"}}
	require.NoError(t, repo.Create(ctx, u2))

	// Try to take u1's identity.
	u2.Identities = append(u2.Identities, u1.Identities[0])
	err := repo.Update(ctx, u2)
	require.ErrorIs(t, err, domain.ErrIdentityTaken)
}

func TestRepository_SetMembershipVersion(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		ctx := t.Context()

		u := newTestUser(t, "mv@example.com")
		require.NoError(t, repo.Create(ctx, u))

		require.NoError(t, repo.SetMembershipVersion(ctx, u.ID, 42))

		got, err := repo.GetByID(ctx, u.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(42), got.MembershipVersion)
	})

	t.Run("missing returns ErrResourceNotFound", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		err := repo.SetMembershipVersion(t.Context(), uuid.New(), 1)
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_SetPassword(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		ctx := t.Context()

		u := newTestUser(t, "pw@example.com")
		require.NoError(t, repo.Create(ctx, u))

		require.NoError(t, repo.SetPassword(ctx, u.ID, "hash-x", true))

		got, err := repo.GetByID(ctx, u.ID)
		require.NoError(t, err)
		assert.Equal(t, "hash-x", got.PasswordHash)
		assert.True(t, got.PasswordChangeRequired)
	})

	t.Run("missing returns ErrResourceNotFound", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		err := repo.SetPassword(t.Context(), uuid.New(), "h", false)
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_GetByIdentity(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		ctx := t.Context()

		u := newTestUser(t, "gi@example.com")
		require.NoError(t, repo.Create(ctx, u))

		got, err := repo.GetByIdentity(ctx, string(u.Identities[0].Provider), u.Identities[0].Subject)
		require.NoError(t, err)
		assert.Equal(t, u.ID, got.ID)
	})

	t.Run("missing returns ErrResourceNotFound", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		_, err := repo.GetByIdentity(t.Context(), "basic", "nope")
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_GetByEmail(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		ctx := t.Context()

		u := newTestUser(t, "ge@example.com")
		require.NoError(t, repo.Create(ctx, u))

		got, err := repo.GetByEmail(ctx, "ge@example.com")
		require.NoError(t, err)
		assert.Equal(t, u.ID, got.ID)
	})

	t.Run("missing returns ErrResourceNotFound", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		_, err := repo.GetByEmail(t.Context(), "nope@example.com")
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_GetSystemUser(t *testing.T) {
	t.Parallel()

	t.Run("returns system user among non-system users", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		ctx := t.Context()

		regular := newTestUser(t, "reg@example.com")
		require.NoError(t, repo.Create(ctx, regular))

		sys := newTestUser(t, "sys@example.com")
		sys.Identities = []domain.Identity{{Provider: domain.ProviderBasic, Subject: "sysident"}}
		sys.System = true
		require.NoError(t, repo.Create(ctx, sys))

		got, err := repo.GetSystemUser(ctx)
		require.NoError(t, err)
		assert.Equal(t, sys.ID, got.ID)
		assert.True(t, got.System)
	})

	t.Run("no system user returns ErrResourceNotFound", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		ctx := t.Context()

		require.NoError(t, repo.Create(ctx, newTestUser(t, "x@example.com")))

		_, err := repo.GetSystemUser(ctx)
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})

	t.Run("empty bucket returns ErrResourceNotFound", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		_, err := repo.GetSystemUser(t.Context())
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

func TestRepository_Delete(t *testing.T) {
	t.Parallel()

	t.Run("happy path cleans indexes", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		ctx := t.Context()

		u := newTestUser(t, "del@example.com")
		require.NoError(t, repo.Create(ctx, u))

		require.NoError(t, repo.Delete(ctx, u.ID))

		_, err := repo.GetByID(ctx, u.ID)
		require.ErrorIs(t, err, storage.ErrResourceNotFound)

		_, err = repo.GetByEmail(ctx, "del@example.com")
		require.ErrorIs(t, err, storage.ErrResourceNotFound)

		_, err = repo.GetByIdentity(ctx, string(u.Identities[0].Provider), u.Identities[0].Subject)
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})

	t.Run("missing returns ErrResourceNotFound", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		err := repo.Delete(t.Context(), uuid.New())
		require.ErrorIs(t, err, storage.ErrResourceNotFound)
	})
}

// seedListUsers creates three users for List tests and returns them so callers
// can reference IDs.
func seedListUsers(t *testing.T, repo *userrepo.Repository) (*domain.User, *domain.User, *domain.User) {
	t.Helper()
	ctx := t.Context()

	alice := newTestUser(t, "alice@example.com")
	alice.DisplayName = "Alice"
	alice.Identities = []domain.Identity{{Provider: domain.ProviderBasic, Subject: "alice"}}
	alice.LastLoginAt = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.Create(ctx, alice))

	bob := newTestUser(t, "bob@example.com")
	bob.DisplayName = "Bob"
	bob.Identities = []domain.Identity{{Provider: domain.ProviderBasic, Subject: "bob"}}
	bob.LastLoginAt = time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.Create(ctx, bob))

	carol := newTestUser(t, "carol@example.com")
	carol.DisplayName = "Carol"
	carol.Identities = []domain.Identity{{Provider: domain.ProviderBasic, Subject: "carol"}}
	carol.LastLoginAt = time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.Create(ctx, carol))

	return alice, bob, carol
}

func TestRepository_List_Filter(t *testing.T) {
	t.Parallel()

	t.Run("AnyUser returns all", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		seedListUsers(t, repo)

		got, total, err := repo.List(t.Context(), domain.UserFilter{AnyUser: true}, domain.UserListParams{})
		require.NoError(t, err)
		assert.Len(t, got, 3)
		assert.Equal(t, 3, total)
	})

	t.Run("UserIDs filter scopes result", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		alice, _, carol := seedListUsers(t, repo)

		filter := domain.UserFilter{
			UserIDs: map[string]struct{}{
				alice.ID.String(): {},
				carol.ID.String(): {},
			},
		}

		got, total, err := repo.List(t.Context(), filter, domain.UserListParams{})
		require.NoError(t, err)
		assert.Len(t, got, 2)
		assert.Equal(t, 2, total)
	})

	t.Run("search matches Email case-insensitively", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		seedListUsers(t, repo)

		got, total, err := repo.List(
			t.Context(),
			domain.UserFilter{AnyUser: true, Search: "BOB"},
			domain.UserListParams{},
		)
		require.NoError(t, err)
		assert.Len(t, got, 1)
		assert.Equal(t, 1, total)
		assert.Equal(t, "bob@example.com", got[0].Email)
	})

	t.Run("search matches DisplayName case-insensitively", func(t *testing.T) {
		t.Parallel()

		repo := newRepo(t)
		seedListUsers(t, repo)

		got, total, err := repo.List(
			t.Context(),
			domain.UserFilter{AnyUser: true, Search: "carol"},
			domain.UserListParams{},
		)
		require.NoError(t, err)
		assert.Len(t, got, 1)
		assert.Equal(t, 1, total)
		assert.Equal(t, "Carol", got[0].DisplayName)
	})
}

func TestRepository_List_Pagination(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	seedListUsers(t, repo)

	got, total, err := repo.List(
		t.Context(),
		domain.UserFilter{AnyUser: true},
		domain.UserListParams{Offset: 1, Limit: 1},
	)
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, 3, total, "total reflects pre-pagination count")
	// Default sort is by Email ascending: alice, bob, carol → offset 1 = bob.
	assert.Equal(t, "bob@example.com", got[0].Email)
}

func TestRepository_List_Sort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sort      domain.SortParams
		wantFirst string
		wantLast  string
	}{
		{
			name:      "default by email asc",
			sort:      domain.SortParams{},
			wantFirst: "alice@example.com",
			wantLast:  "carol@example.com",
		},
		{
			name:      "default by email desc",
			sort:      domain.SortParams{Desc: true},
			wantFirst: "carol@example.com",
			wantLast:  "alice@example.com",
		},
		{
			name:      "by name asc",
			sort:      domain.SortParams{Field: "name"},
			wantFirst: "alice@example.com", // Alice < Bob < Carol
			wantLast:  "carol@example.com",
		},
		{
			name:      "by last_login asc",
			sort:      domain.SortParams{Field: "last_login"},
			wantFirst: "alice@example.com", // earliest LastLoginAt
			wantLast:  "carol@example.com",
		},
		{
			name:      "by last_login desc",
			sort:      domain.SortParams{Field: "last_login", Desc: true},
			wantFirst: "carol@example.com",
			wantLast:  "alice@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := newRepo(t)
			seedListUsers(t, repo)

			got, _, err := repo.List(
				t.Context(),
				domain.UserFilter{AnyUser: true},
				domain.UserListParams{Sort: tt.sort},
			)
			require.NoError(t, err)
			require.Len(t, got, 3)
			assert.Equal(t, tt.wantFirst, got[0].Email)
			assert.Equal(t, tt.wantLast, got[2].Email)
		})
	}
}

func TestRepository_ListAll(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	seedListUsers(t, repo)

	got, err := repo.ListAll(t.Context())
	require.NoError(t, err)
	assert.Len(t, got, 3)
}
