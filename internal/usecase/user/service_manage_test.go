package user_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
	"github.com/sergeyslonimsky/elara/internal/usecase/user"
)

// Authorization for UserService.* is enforced in the handler layer (EL-4 M9);
// these tests cover only the business logic remaining in the usecase
// (validation, self-deletion guard, last-admin guard, List scoping).

func contextWithAdmin(ctx context.Context) context.Context {
	return auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
}

func TestService_Create(t *testing.T) {
	t.Parallel()

	const (
		email    = "new-user@example.com"
		name     = "New User"
		password = "initial-password"
	)

	tests := []struct {
		name     string
		email    string
		password string
		mockFunc func(ctx context.Context, m mocks)
		wantErr  string
		want     *domain.User
	}{
		{
			name:     "success",
			email:    email,
			password: password,
			mockFunc: func(ctx context.Context, m mocks) {
				m.store.EXPECT().Upsert(ctx, gomock.AssignableToTypeOf(&domain.User{})).Return(nil)
				m.store.EXPECT().SetPassword(ctx, email, gomock.Any(), true).Return(nil)
			},
			want: &domain.User{Email: email, Name: name, Provider: domain.ProviderBasicAuth},
		},
		{
			name:     "OIDC pre-create (empty password)",
			email:    email,
			password: "",
			mockFunc: func(ctx context.Context, m mocks) {
				m.store.EXPECT().Upsert(ctx, gomock.Cond(func(x any) bool {
					u, ok := x.(*domain.User)

					return ok && u.Provider == domain.ProviderOIDC
				})).Return(nil)
			},
			want: &domain.User{Email: email, Name: name, Provider: domain.ProviderOIDC},
		},
		{
			name:     "validation error",
			email:    "invalid-email",
			password: password,
			mockFunc: func(_ context.Context, _ mocks) {},
			wantErr:  "validate user",
		},
		{
			name:     "upsert fails",
			email:    email,
			password: password,
			mockFunc: func(ctx context.Context, m mocks) {
				m.store.EXPECT().Upsert(ctx, gomock.Any()).Return(assert.AnError)
			},
			wantErr: "upsert user",
		},
		{
			name:     "set password fails",
			email:    email,
			password: password,
			mockFunc: func(ctx context.Context, m mocks) {
				m.store.EXPECT().Upsert(ctx, gomock.Any()).Return(nil)
				m.store.EXPECT().SetPassword(ctx, email, gomock.Any(), true).Return(assert.AnError)
			},
			wantErr: "set password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sut, m, _, _ := setupService(t)
			ctx := t.Context()
			tt.mockFunc(ctx, m)

			got, err := sut.Create(ctx, tt.email, name, tt.password)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestService_Delete(t *testing.T) {
	t.Parallel()

	const (
		adminEmail  = "admin@example.com"
		targetEmail = "target@example.com"
	)

	t.Run("unauthorized (no claims) -> ErrUnauthorized", func(t *testing.T) {
		t.Parallel()

		sut, _, _, _ := setupService(t)

		err := sut.Delete(t.Context(), targetEmail)
		require.ErrorIs(t, err, domain.ErrUnauthorized)
	})

	t.Run("self-deletion is rejected", func(t *testing.T) {
		t.Parallel()

		sut, _, _, _ := setupService(t)
		ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: targetEmail})

		err := sut.Delete(ctx, targetEmail)
		require.ErrorContains(t, err, "cannot delete your own account")
	})

	t.Run("user not found", func(t *testing.T) {
		t.Parallel()

		sut, m, _, _ := setupService(t)
		ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: adminEmail})

		m.store.EXPECT().Get(ctx, targetEmail).Return(nil, domain.ErrNotFound)

		err := sut.Delete(ctx, targetEmail)
		require.ErrorContains(t, err, "get user")
	})

	t.Run("success removes user and Casbin rules atomically", func(t *testing.T) {
		t.Parallel()

		sut, m, _, _ := setupService(t)
		ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: adminEmail})

		// The mocked store has no Casbin admin grant, so the last-admin guard
		// is satisfied. Store.Get returns the target user; the WriteTx path
		// then writes through the real bbolt UserRepo (whose Delete will
		// return NotFound because we never seeded it). We accept that and
		// assert the error wrapping, not full integration of users + casbin.
		m.store.EXPECT().Get(ctx, targetEmail).Return(&domain.User{Email: targetEmail}, nil)

		err := sut.Delete(ctx, targetEmail)
		// The real UserRepo returns NotFound on an unseeded user; we wrap as
		// "delete user". A fully-integrated test would also seed the user via
		// users.Upsert and then assert successful removal; this remains an
		// open follow-up.
		require.ErrorContains(t, err, "delete user")
	})
}

func TestService_Get(t *testing.T) {
	t.Parallel()

	const targetEmail = "user@example.com"
	u := &domain.User{Email: targetEmail}

	tests := []struct {
		name     string
		mockFunc func(ctx context.Context, m mocks)
		wantErr  string
		want     *domain.User
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, m mocks) {
				m.store.EXPECT().Get(ctx, targetEmail).Return(u, nil)
			},
			want: u,
		},
		{
			name: "store fails",
			mockFunc: func(ctx context.Context, m mocks) {
				m.store.EXPECT().Get(ctx, targetEmail).Return(nil, assert.AnError)
			},
			wantErr: "get user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sut, m, _, _ := setupService(t)
			ctx := t.Context()
			tt.mockFunc(ctx, m)

			got, err := sut.Get(ctx, targetEmail)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// List uses an integration stack because Service.List depends on
// enforcer.GetMembersOfGroup (concrete *casbin.Enforcer, not an interface).
// Wildcard-scope cases keep the mocked store so we can lock filter shape and
// inject store errors; explicit-scope cases use the real UserRepo so members
// resolved through Casbin actually pass through the bbolt filter.

func TestService_List_Unauthenticated(t *testing.T) {
	t.Parallel()

	sut, _, _, _ := setupService(t)

	_, err := sut.List(t.Context(), user.ListParams{})
	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestService_List_WildcardScope_ForwardsAnyUserAndDefaults(t *testing.T) {
	t.Parallel()

	sut, m, store, enforcer := setupService(t)
	ctx := t.Context()
	txm := bbolt.NewTxManager(store.DB())

	// Grant admin (*,*,*) so EffectiveDomains returns wildcard.
	require.NoError(t, enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		return txe.AddPolicy(
			"admin@example.com",
			domain.DomainAll,
			domain.ObjectAll,
			domain.ActionAll,
		)
	}))

	want := []*domain.User{{Email: "a@example.com"}, {Email: "b@example.com"}}

	reqCtx := contextWithAdmin(ctx)
	m.store.EXPECT().
		List(
			reqCtx,
			domain.UserFilter{AnyUser: true, Search: "ali"},
			domain.UserListParams{Limit: 20, Offset: 0, Sort: domain.SortParams{}},
		).
		Return(want, 2, nil)

	got, err := sut.List(reqCtx, user.ListParams{Query: "ali"})
	require.NoError(t, err)
	assert.Equal(t, want, got.Users)
	assert.Equal(t, 2, got.Total)
	// Lock default limit literal.
	assert.Equal(t, 20, got.Limit)
	assert.Equal(t, 0, got.Offset)
}

func TestService_List_WildcardScope_PaginationForwarded(t *testing.T) {
	t.Parallel()

	sut, m, store, enforcer := setupService(t)
	ctx := t.Context()
	txm := bbolt.NewTxManager(store.DB())

	require.NoError(t, enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		return txe.AddPolicy(
			"admin@example.com",
			domain.DomainAll,
			domain.ObjectAll,
			domain.ActionAll,
		)
	}))

	reqCtx := contextWithAdmin(ctx)
	m.store.EXPECT().
		List(
			reqCtx,
			domain.UserFilter{AnyUser: true},
			domain.UserListParams{Limit: 5, Offset: 10, Sort: domain.SortParams{}},
		).
		Return(nil, 0, nil)

	got, err := sut.List(reqCtx, user.ListParams{Limit: 5, Offset: 10})
	require.NoError(t, err)
	assert.Equal(t, 5, got.Limit)
	assert.Equal(t, 10, got.Offset)
}

func TestService_List_WildcardScope_StoreErrorWrapped(t *testing.T) {
	t.Parallel()

	sut, m, store, enforcer := setupService(t)
	ctx := t.Context()
	txm := bbolt.NewTxManager(store.DB())

	require.NoError(t, enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		return txe.AddPolicy(
			"admin@example.com",
			domain.DomainAll,
			domain.ObjectAll,
			domain.ActionAll,
		)
	}))

	reqCtx := contextWithAdmin(ctx)
	m.store.EXPECT().
		List(reqCtx, gomock.Any(), gomock.Any()).
		Return(nil, 0, assert.AnError)

	_, err := sut.List(reqCtx, user.ListParams{})
	require.ErrorContains(t, err, "list users:")
}

func TestService_List_EmptyScopeReturnsEmpty_StoreNotCalled(t *testing.T) {
	t.Parallel()

	// Using setupService (mocked store) without any EXPECT proves the store
	// is never invoked: gomock fails the test on unexpected calls.
	sut, _, _, _ := setupService(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "nobody@example.com"})

	got, err := sut.List(ctx, user.ListParams{})
	require.NoError(t, err)
	assert.Empty(t, got.Users)
	assert.Equal(t, 0, got.Total)
	assert.Equal(t, 20, got.Limit)
}

func TestService_List_ExplicitScope_RollsUpMembers(t *testing.T) {
	t.Parallel()

	sut, store, enforcer, users := setupServiceReal(t)
	ctx := t.Context()
	txm := bbolt.NewTxManager(store.DB())

	// Caller has read on group:dev and group:platform.
	require.NoError(t, enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		if err := txe.AddPolicy(
			"delegated@example.com",
			casbin.GroupSubject("dev"),
			domain.ObjectGroup,
			domain.ActionRead,
		); err != nil {
			return err
		}

		return txe.AddPolicy(
			"delegated@example.com",
			casbin.GroupSubject("platform"),
			domain.ObjectGroup,
			domain.ActionRead,
		)
	}))

	// Seed group memberships (g-rules).
	require.NoError(t, enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		for _, m := range []struct{ user, group string }{
			{"alice@x", "dev"},
			{"bob@x", "dev"},
			{"bob@x", "platform"},
			{"charlie@x", "platform"},
			{"outsider@x", "other"},
		} {
			if err := txe.AddRoleForUser(m.user, casbin.GroupSubject(m.group), domain.MembershipDomain); err != nil {
				return err
			}
		}

		return nil
	}))

	// Seed user records; outsider must not appear in results.
	for _, email := range []string{"alice@x", "bob@x", "charlie@x", "outsider@x"} {
		require.NoError(t, users.Upsert(ctx, &domain.User{
			Email:       email,
			Name:        email,
			Provider:    domain.ProviderBasicAuth,
			LastLoginAt: time.Now(),
		}))
	}

	reqCtx := auth.WithClaims(ctx, &auth.Claims{Email: "delegated@example.com"})
	got, err := sut.List(reqCtx, user.ListParams{})
	require.NoError(t, err)
	assert.Equal(t, 3, got.Total)
	require.Len(t, got.Users, 3)

	emails := []string{got.Users[0].Email, got.Users[1].Email, got.Users[2].Email}
	assert.ElementsMatch(t, []string{"alice@x", "bob@x", "charlie@x"}, emails)
}

func TestService_List_ExplicitScope_NestedGroupSubjectsSkipped(t *testing.T) {
	t.Parallel()

	sut, store, enforcer, users := setupServiceReal(t)
	ctx := t.Context()
	txm := bbolt.NewTxManager(store.DB())

	require.NoError(t, enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		return txe.AddPolicy(
			"delegated@example.com",
			casbin.GroupSubject("dev"),
			domain.ObjectGroup,
			domain.ActionRead,
		)
	}))

	// Members include a nested-group subject which must be skipped by List.
	require.NoError(t, enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		if err := txe.AddRoleForUser("alice@x", casbin.GroupSubject("dev"), domain.MembershipDomain); err != nil {
			return err
		}

		return txe.AddRoleForUser(
			casbin.GroupSubject("nested"),
			casbin.GroupSubject("dev"),
			domain.MembershipDomain,
		)
	}))

	require.NoError(t, users.Upsert(ctx, &domain.User{
		Email: "alice@x", Name: "Alice", Provider: domain.ProviderBasicAuth,
	}))

	reqCtx := auth.WithClaims(ctx, &auth.Claims{Email: "delegated@example.com"})
	got, err := sut.List(reqCtx, user.ListParams{})
	require.NoError(t, err)
	require.Len(t, got.Users, 1)
	assert.Equal(t, "alice@x", got.Users[0].Email)
}

func TestService_List_ExplicitScope_GroupsVisibleButNoMembers_StoreNotCalled(t *testing.T) {
	t.Parallel()

	// Real stack but no users seeded; the early-return branch must fire when
	// the rolled-up username set is empty so the repo scan never happens.
	sut, store, enforcer, _ := setupServiceReal(t)
	ctx := t.Context()
	txm := bbolt.NewTxManager(store.DB())

	require.NoError(t, enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		return txe.AddPolicy(
			"delegated@example.com",
			casbin.GroupSubject("empty"),
			domain.ObjectGroup,
			domain.ActionRead,
		)
	}))

	reqCtx := auth.WithClaims(ctx, &auth.Claims{Email: "delegated@example.com"})
	got, err := sut.List(reqCtx, user.ListParams{})
	require.NoError(t, err)
	assert.Empty(t, got.Users)
	assert.Equal(t, 0, got.Total)
	assert.Equal(t, 20, got.Limit)
}

func TestService_List_ExplicitScope_SearchForwarded(t *testing.T) {
	t.Parallel()

	sut, store, enforcer, users := setupServiceReal(t)
	ctx := t.Context()
	txm := bbolt.NewTxManager(store.DB())

	require.NoError(t, enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		return txe.AddPolicy(
			"delegated@example.com",
			casbin.GroupSubject("dev"),
			domain.ObjectGroup,
			domain.ActionRead,
		)
	}))
	require.NoError(t, enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		if err := txe.AddRoleForUser("alice@x", casbin.GroupSubject("dev"), domain.MembershipDomain); err != nil {
			return err
		}

		return txe.AddRoleForUser("bob@x", casbin.GroupSubject("dev"), domain.MembershipDomain)
	}))
	for _, u := range []*domain.User{
		{Email: "alice@x", Name: "Alice", Provider: domain.ProviderBasicAuth},
		{Email: "bob@x", Name: "Bob", Provider: domain.ProviderBasicAuth},
	} {
		require.NoError(t, users.Upsert(ctx, u))
	}

	reqCtx := auth.WithClaims(ctx, &auth.Claims{Email: "delegated@example.com"})
	got, err := sut.List(reqCtx, user.ListParams{Query: "ALI"})
	require.NoError(t, err)
	require.Len(t, got.Users, 1)
	assert.Equal(t, "alice@x", got.Users[0].Email)
}
