package user_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/usecase/user"
	usermock "github.com/sergeyslonimsky/elara/internal/usecase/user/mocks"
)

type mocks struct {
	store *usermock.Mockstore
}

// setupService wires a Service with a mocked store and real (bbolt-backed)
// Enforcer + UserRepo + TxManager + PDP. Paths that go through Enforcer.WriteTx
// (Delete) exercise the integration helper end-to-end; pure store paths
// (Create, Get, ResetPassword, plus the wildcard branch of List) drive the
// mocked store. The real *bbolt.Store and *casbin.Enforcer are returned so
// individual tests can seed Casbin policies (e.g. wildcard grants) directly.
func setupService(t *testing.T) (*user.Service, mocks, *bbolt.Store, *casbin.Enforcer) {
	t.Helper()
	ctrl := gomock.NewController(t)

	path := filepath.Join(t.TempDir(), "user.db")

	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	policies := bbolt.NewPolicyRepo(store)
	enforcer, err := casbin.NewEnforcer(policies)
	require.NoError(t, err)

	users := bbolt.NewUserRepo(store)
	txm := bbolt.NewTxManager(store.DB())
	pdp := authz.NewPDP(enforcer)

	m := mocks{store: usermock.NewMockstore(ctrl)}

	return user.New(enforcer, m.store, users, txm, pdp), m, store, enforcer
}

// setupServiceReal boots the full integration stack (no store mock). Used by
// List tests where Service.List walks enforcer.GetMembersOfGroup and the real
// bbolt UserRepo. Seed users via the returned UserRepo and Casbin grants via
// the returned enforcer + Store.
func setupServiceReal(t *testing.T) (*user.Service, *bbolt.Store, *casbin.Enforcer, *bbolt.UserRepo) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "user.db")

	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	policies := bbolt.NewPolicyRepo(store)
	enforcer, err := casbin.NewEnforcer(policies)
	require.NoError(t, err)

	users := bbolt.NewUserRepo(store)
	txm := bbolt.NewTxManager(store.DB())
	pdp := authz.NewPDP(enforcer)

	return user.New(enforcer, users, users, txm, pdp), store, enforcer, users
}
