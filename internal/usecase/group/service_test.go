package group_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/usecase/group"
)

// newTestService boots a real bbolt store + Casbin enforcer + GroupRepo +
// TxManager and returns a wired Service. Tests use the real stack rather
// than mocks because the service depends on concrete per-tx views
// (Enforcer.WithTx, GroupRepo.WithTx) whose return types do not fit
// cleanly into mockable interfaces. The integration helper also exercises
// the §4 level-2 atomicity invariant end-to-end.
func newTestService(t *testing.T) (*group.Service, *bbolt.Store, *casbin.Enforcer, *bbolt.GroupRepo) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "group.db")

	store, err := bbolt.Open(path)
	require.NoError(t, err)

	t.Cleanup(func() { _ = store.Close() })

	policies := bbolt.NewPolicyRepo(store)
	enforcer, err := casbin.NewEnforcer(policies)
	require.NoError(t, err)

	groups := bbolt.NewGroupRepo(store)
	txm := bbolt.NewTxManager(store.DB())
	pdp := authz.NewPDP(enforcer)

	return group.New(enforcer, groups, txm, pdp), store, enforcer, groups
}
