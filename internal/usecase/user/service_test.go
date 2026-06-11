package user_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/auth/sessions"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	grouprepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/group"
	sessionrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/session"
	userrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/user"
	"github.com/sergeyslonimsky/elara/internal/usecase/user"
	usermock "github.com/sergeyslonimsky/elara/internal/usecase/user/mocks"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
	"github.com/sergeyslonimsky/elara/test/bbolttest"
)

// ---- common test identities ---------------------------------------------------

const (
	adminEmail    = "admin@example.com"
	actorEmail    = "actor@example.com"
	targetEmail   = "target@example.com"
	resetPassword = "S3cret-Reset-Pwd"

	// adminID / actorID / targetID are the stable user IDs used in tests.
	// Production code resolves actor.UserID as the Casbin subject for policy
	// enforcement, and stores membership g-rules under user.ID (UUID). Both the
	// AuthInfo and the bbolt seed must agree on the same value.
	adminID  = "00000000-0000-0000-0000-000000000001"
	actorID  = "00000000-0000-0000-0000-000000000002"
	targetID = "00000000-0000-0000-0000-000000000003"
)

// uuid-typed mirrors of the string IDs above. Usecase methods take uuid.UUID
// for user targeting (handler parses the wire-level string); the string form
// is still used for Casbin subjects and AuthInfo.UserID.
var (
	adminUUID  = uuid.MustParse(adminID)
	targetUUID = uuid.MustParse(targetID)
	// ghostUUID points at no user — tests use it for "not found" assertions.
	ghostUUID = uuid.MustParse("00000000-0000-0000-0000-0000000000ff")
)

func adminActor() domain.AuthInfo { return domain.AuthInfo{UserID: adminID, Email: adminEmail} }
func actor() domain.AuthInfo      { return domain.AuthInfo{UserID: actorID, Email: actorEmail} }

// ---- real integration stack --------------------------------------------------

// realStack bundles every dependency a test might need to seed bbolt and
// Casbin in addition to driving the Service under test. It is the integration
// entry-point used by every happy-path / authz test in this package.
type realStack struct {
	svc         *user.Service
	store       *bbolt.Store
	enforcer    *casbin.Enforcer
	users       *userrepo.Repository
	groups      *grouprepo.Repository
	txm         *bbolt.Manager
	pkgManager  pkgbbolt.Manager
	sessionRepo *sessionrepo.Repository
}

// setupServiceReal boots the full integration stack — real bbolt + real
// Casbin enforcer + real authz Scope. Use this for assertions that observe
// actual persistence and Casbin g-rules rather than mock-interaction counts.
func setupServiceReal(t *testing.T) realStack {
	t.Helper()

	stack := bbolttest.OpenStack(t)
	txm := stack.Txm
	users := userrepo.NewRepository(stack.PkgManager)
	groupRepo := grouprepo.NewRepository(stack.PkgManager)
	sessionRepo := sessionrepo.NewRepository(stack.PkgManager)
	sessionEventRepo := sessionrepo.NewEventRepository(stack.PkgManager)
	sessionSvc := sessions.New(sessionRepo, sessionEventRepo, sessions.RealClock{})

	pdp := authz.NewPDP(stack.Enforcer)
	pap := authz.NewPAP(stack.Enforcer, txm)
	scope := authz.NewScope(pdp, pap, groupRepo)

	userSvc := auth.NewUserService(users)

	return realStack{
		svc:         user.New(txm, users, userSvc, groupRepo, sessionSvc, pdp, pap, scope),
		store:       stack.Store,
		enforcer:    stack.Enforcer,
		users:       users,
		groups:      groupRepo,
		txm:         txm,
		pkgManager:  stack.PkgManager,
		sessionRepo: sessionRepo,
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

	require.NoError(
		t,
		st.enforcer.WriteTx(t.Context(), st.txm, func(ctx context.Context, txe *casbin.TxEnforcer) error {
			for _, r := range rules {
				if err := txe.AddPolicy(r.Sub, r.Dom, string(r.Obj), string(r.Act)); err != nil {
					return err
				}
			}

			return nil
		}),
	)
}

// addMemberships writes membership g-rules (`user, group:<name>, *`) inside a
// Casbin write transaction. Caller passes rows of (user, groupName).
func addMemberships(t *testing.T, st realStack, rows []struct{ User, GroupName string }) {
	t.Helper()

	require.NoError(
		t,
		st.enforcer.WriteTx(t.Context(), st.txm, func(ctx context.Context, txe *casbin.TxEnforcer) error {
			for _, m := range rows {
				subject := domain.GroupResource(m.GroupName)
				if err := txe.AddRoleForUser(m.User, subject, domain.MembershipDomain); err != nil {
					return err
				}
			}

			return nil
		}),
	)
}

// seedAdminAll grants admin@example.com the (*,*,*) policy. Most usecase
// tests want this so that authorization checks resolve to true and the
// test focuses on the surrounding business logic.
func seedAdminAll(t *testing.T, st realStack) {
	t.Helper()

	addPolicies(t, st, []policyRow{
		{adminID, domain.DomainAll, domain.ObjectAll, domain.ActionAll},
	})
}

// seedUser writes a baseline user into bbolt via the real UserRepo.
// id is optional — pass "" to let bbolt mint a UUID.
func seedUser(t *testing.T, st realStack, email string) {
	t.Helper()
	seedUserWithID(t, st, "", email)
}

// seedUserWithID writes a baseline user with a known stable ID into bbolt.
// Use this when the test needs actor.UserID to match the persisted user.ID.
func seedUserWithID(t *testing.T, st realStack, id, email string) {
	t.Helper()

	var uid uuid.UUID
	if id != "" {
		var err error
		uid, err = uuid.Parse(id)
		require.NoError(t, err)
	}

	if uid == uuid.Nil {
		uid = uuid.New()
	}

	require.NoError(t, st.users.Create(t.Context(), &domain.User{
		ID:          uid,
		Email:       email,
		DisplayName: email,
		Status:      domain.UserStatusActive,
		Identities: []domain.Identity{
			{Provider: domain.ProviderBasic, Subject: email},
		},
	}))
}

// seedGroup writes a baseline group into bbolt via the real GroupRepo.
func seedGroup(t *testing.T, st realStack, _, name string) {
	t.Helper()

	require.NoError(t, st.groups.Create(t.Context(), &domain.Group{Name: name}))
}

// ---- mock stack (fault injection only) ---------------------------------------

// mockStack wires a Service whose UserReader is a gomock. Use it ONLY for
// fault-injection tests where we want to lock a specific error-wrapping
// message — e.g. forcing Upsert/Get/SetPassword to return a contrived error.
// Happy paths and authz assertions belong to setupServiceReal.
type mockStack struct {
	svc      *user.Service
	store    *usermock.MockUserReader
	users    *usermock.MockUserManager
	sessions *usermock.MocksessionsService
	bolt     *bbolt.Store
	enforcer *casbin.Enforcer
	txm      *bbolt.Manager
}

func setupServiceWithMockStore(t *testing.T) mockStack {
	t.Helper()
	ctrl := gomock.NewController(t)

	stack := bbolttest.OpenStack(t)
	txm := stack.Txm
	groupRepo := grouprepo.NewRepository(stack.PkgManager)
	sessionSvc := usermock.NewMocksessionsService(ctrl)

	pdp := authz.NewPDP(stack.Enforcer)
	pap := authz.NewPAP(stack.Enforcer, txm)
	scope := authz.NewScope(pdp, pap, groupRepo)

	mockStore := usermock.NewMockUserReader(ctrl)
	mockUsers := usermock.NewMockUserManager(ctrl)

	return mockStack{
		svc:      user.New(txm, mockStore, mockUsers, groupRepo, sessionSvc, pdp, pap, scope),
		store:    mockStore,
		users:    mockUsers,
		sessions: sessionSvc,
		bolt:     stack.Store,
		enforcer: stack.Enforcer,
		txm:      txm,
	}
}

// seedAdminAllOnMockStack grants the admin (*,*,*) on a mock-store-backed
// service. Pulled out so fault-injection tests don't duplicate the dance.
func seedAdminAllOnMockStack(t *testing.T, m mockStack) {
	t.Helper()

	require.NoError(
		t,
		m.enforcer.WriteTx(t.Context(), m.txm, func(ctx context.Context, txe *casbin.TxEnforcer) error {
			return txe.AddPolicy(
				adminID,
				domain.DomainAll,
				string(domain.ObjectAll),
				string(domain.ActionAll),
			)
		}),
	)
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

		// Actor can read groups: dev and platform. Policy subject is actorID.
		addPolicies(t, st, []policyRow{
			{actorID, domain.GroupResource("dev"), domain.ObjectGroup, domain.ActionRead},
			{actorID, domain.GroupResource("platform"), domain.ObjectGroup, domain.ActionRead},
		})

		// Seed users with stable IDs so membership g-rules match user.ID.
		seedUserWithID(t, st, "00000000-0000-0000-0000-000000000011", "alice@x")
		seedUserWithID(t, st, "00000000-0000-0000-0000-000000000012", "bob@x")
		seedUserWithID(t, st, "00000000-0000-0000-0000-000000000013", "outsider@x")

		// Memberships stored under user.ID (UUID) in Casbin.
		addMemberships(t, st, []struct{ User, GroupName string }{
			{"00000000-0000-0000-0000-000000000011", "dev"},
			{"00000000-0000-0000-0000-000000000012", "platform"},
			{"00000000-0000-0000-0000-000000000013", "other"},
		})

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
