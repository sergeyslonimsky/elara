package user_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
	"github.com/sergeyslonimsky/elara/internal/usecase/user"
	usermock "github.com/sergeyslonimsky/elara/internal/usecase/user/mocks"
	"github.com/sergeyslonimsky/elara/test/bbolttest"
)

// ---- common test identities ---------------------------------------------------

const (
	adminEmail    = "admin@example.com"
	actorEmail    = "actor@example.com"
	targetEmail   = "target@example.com"
	resetPassword = "S3cret-Reset-Pwd"
)

func adminActor() domain.AuthInfo { return domain.AuthInfo{Email: adminEmail} }
func actor() domain.AuthInfo      { return domain.AuthInfo{Email: actorEmail} }

// ---- real integration stack --------------------------------------------------

// realStack bundles every dependency a test might need to seed bbolt and
// Casbin in addition to driving the Service under test. It is the integration
// entry-point used by every happy-path / authz test in this package.
type realStack struct {
	svc      *user.Service
	store    *bbolt.Store
	enforcer *casbin.Enforcer
	users    *bbolt.UserRepo
	groups   *bbolt.GroupRepo
	txm      *bbolt.TxManager
}

// setupServiceReal boots the full integration stack — real bbolt + real
// Casbin enforcer + real authz Scope. Use this for assertions that observe
// actual persistence and Casbin g-rules rather than mock-interaction counts.
func setupServiceReal(t *testing.T) realStack {
	t.Helper()

	store, enforcer, txm := bbolttest.OpenStack(t)
	users := bbolt.NewUserRepo(store)
	groupRepo := bbolt.NewGroupRepo(store)
	groups := user.NewBoltGroupReader(groupRepo)
	pdp := authz.NewPDP(enforcer)
	pap := authz.NewPAP(enforcer, txm)
	scope := authz.NewScope(pdp, pap, groupRepo)

	return realStack{
		svc:      user.New(user.NewBoltUserReader(users), groups, pdp, pap, scope),
		store:    store,
		enforcer: enforcer,
		users:    users,
		groups:   groupRepo,
		txm:      txm,
	}
}

// policyRow is a single p-rule row passed to addPolicies. Object/Action
// are typed so call sites can use domain.ObjectXxx / domain.ActionXxx
// constants without string casts.
type policyRow struct {
	Sub string
	Dom string
	Obj domain.Object
	Act domain.Action
}

// addPolicies writes p-rules (subject, domain, object, action) inside a
// Casbin write transaction.
func addPolicies(t *testing.T, st realStack, rules []policyRow) {
	t.Helper()

	require.NoError(t, st.enforcer.WriteTx(t.Context(), st.txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		for _, r := range rules {
			if err := txe.AddPolicy(r.Sub, r.Dom, string(r.Obj), string(r.Act)); err != nil {
				return err
			}
		}

		return nil
	}))
}

// addMemberships writes membership g-rules (`user, group:<name>, *`) inside a
// Casbin write transaction. Caller passes rows of (user, groupName).
func addMemberships(t *testing.T, st realStack, rows []struct{ User, GroupName string }) {
	t.Helper()

	require.NoError(t, st.enforcer.WriteTx(t.Context(), st.txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		for _, m := range rows {
			subject := casbin.GroupSubject(m.GroupName)
			if err := txe.AddRoleForUser(m.User, subject, domain.MembershipDomain); err != nil {
				return err
			}
		}

		return nil
	}))
}

// seedAdminAll grants admin@example.com the (*,*,*) policy. Most usecase
// tests want this so that authorization checks resolve to true and the
// test focuses on the surrounding business logic.
func seedAdminAll(t *testing.T, st realStack) {
	t.Helper()

	addPolicies(t, st, []policyRow{
		{adminEmail, domain.DomainAll, domain.ObjectAll, domain.ActionAll},
	})
}

// seedUser writes a baseline user into bbolt via the real UserRepo.
func seedUser(t *testing.T, st realStack, email string) {
	t.Helper()

	require.NoError(t, st.users.Upsert(t.Context(), &domain.User{
		Email:    email,
		Name:     email,
		Provider: domain.ProviderBasicAuth,
	}))
}

// seedGroup writes a baseline group into bbolt via the real GroupRepo.
func seedGroup(t *testing.T, st realStack, id, name string) {
	t.Helper()

	require.NoError(t, st.groups.Create(t.Context(), &domain.Group{ID: id, Name: name}))
}

// ---- mock stack (fault injection only) ---------------------------------------

// mockStack wires a Service whose UserReader is a gomock. Use it ONLY for
// fault-injection tests where we want to lock a specific error-wrapping
// message — e.g. forcing Upsert/Get/SetPassword to return a contrived error.
// Happy paths and authz assertions belong to setupServiceReal.
type mockStack struct {
	svc      *user.Service
	store    *usermock.MockUserReader
	bolt     *bbolt.Store
	enforcer *casbin.Enforcer
	txm      *bbolt.TxManager
}

func setupServiceWithMockStore(t *testing.T) mockStack {
	t.Helper()
	ctrl := gomock.NewController(t)

	store, enforcer, txm := bbolttest.OpenStack(t)
	groupRepo := bbolt.NewGroupRepo(store)
	groups := user.NewBoltGroupReader(groupRepo)
	pdp := authz.NewPDP(enforcer)
	pap := authz.NewPAP(enforcer, txm)
	scope := authz.NewScope(pdp, pap, groupRepo)

	mockStore := usermock.NewMockUserReader(ctrl)
	// WithTx returns the same mock so expectations recorded on the parent
	// cover both pre-tx and in-tx call paths.
	mockStore.EXPECT().WithTx(gomock.Any()).AnyTimes().Return(mockStore)

	return mockStack{
		svc:      user.New(mockStore, groups, pdp, pap, scope),
		store:    mockStore,
		bolt:     store,
		enforcer: enforcer,
		txm:      txm,
	}
}

// seedAdminAllOnMockStack grants the admin (*,*,*) on a mock-store-backed
// service. Pulled out so fault-injection tests don't duplicate the dance.
func seedAdminAllOnMockStack(t *testing.T, m mockStack) {
	t.Helper()

	require.NoError(t, m.enforcer.WriteTx(t.Context(), m.txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		return txe.AddPolicy(adminEmail, domain.DomainAll, string(domain.ObjectAll), string(domain.ActionAll))
	}))
}

// ---- List ---------------------------------------------------------------------

func TestService_List(t *testing.T) {
	t.Parallel()

	t.Run("global User:Read returns everyone with pagination defaults", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)
		seedUser(t, st, "alice@example.com")
		seedUser(t, st, "bob@example.com")

		got, err := st.svc.List(t.Context(), adminActor(), user.ListParams{})
		require.NoError(t, err)
		assert.Equal(t, 2, got.Total)
		assert.Equal(t, 20, got.Limit)
		assert.Equal(t, 0, got.Offset)
		assert.Len(t, got.Users, 2)
	})

	t.Run("search filter forwarded to store", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)
		seedUser(t, st, "alice@example.com")
		seedUser(t, st, "bob@example.com")

		got, err := st.svc.List(t.Context(), adminActor(), user.ListParams{Query: "ali"})
		require.NoError(t, err)
		require.Len(t, got.Users, 1)
		assert.Equal(t, "alice@example.com", got.Users[0].Email)
	})

	t.Run("pagination forwarded to store", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)
		for _, e := range []string{"a@x.io", "b@x.io", "c@x.io"} {
			seedUser(t, st, e)
		}

		got, err := st.svc.List(t.Context(), adminActor(), user.ListParams{Limit: 1, Offset: 1})
		require.NoError(t, err)
		assert.Equal(t, 1, got.Limit)
		assert.Equal(t, 1, got.Offset)
		assert.Equal(t, 3, got.Total)
		assert.Len(t, got.Users, 1)
	})

	t.Run("empty scope returns empty without enumerating users", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		// Seed several users the actor must NOT see — no grants, no
		// memberships. Empty scope must short-circuit the store call.
		seedUser(t, st, "alice@example.com")
		seedUser(t, st, "bob@example.com")

		got, err := st.svc.List(t.Context(), actor(), user.ListParams{})
		require.NoError(t, err)
		assert.Empty(t, got.Users)
		assert.Equal(t, 0, got.Total)
		assert.Equal(t, 20, got.Limit)
	})

	t.Run("derived through Group:Read scope rolls up only group members", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)

		// Actor can read groups: dev and platform.
		addPolicies(t, st, []policyRow{
			{actorEmail, casbin.GroupSubject("dev"), domain.ObjectGroup, domain.ActionRead},
			{actorEmail, casbin.GroupSubject("platform"), domain.ObjectGroup, domain.ActionRead},
		})

		addMemberships(t, st, []struct{ User, GroupName string }{
			{"alice@x", "dev"},
			{"bob@x", "platform"},
			{"outsider@x", "other"},
		})

		for _, email := range []string{"alice@x", "bob@x", "outsider@x"} {
			seedUser(t, st, email)
		}

		got, err := st.svc.List(t.Context(), actor(), user.ListParams{})
		require.NoError(t, err)
		require.Len(t, got.Users, 2)
		emails := []string{got.Users[0].Email, got.Users[1].Email}
		assert.ElementsMatch(t, []string{"alice@x", "bob@x"}, emails)
	})
}

// TestService_List_StoreErrorWrapped verifies the error wrapping the List
// usecase applies when the underlying store fails. Fault injection — the
// only place a mock store is justified for this method.
func TestService_List_StoreErrorWrapped(t *testing.T) {
	t.Parallel()

	m := setupServiceWithMockStore(t)
	seedAdminAllOnMockStack(t, m)

	m.store.EXPECT().
		List(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, 0, assert.AnError)

	_, err := m.svc.List(t.Context(), adminActor(), user.ListParams{})
	require.ErrorContains(t, err, "list users")
}
