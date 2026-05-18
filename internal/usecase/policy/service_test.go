package policy_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
	"github.com/sergeyslonimsky/elara/internal/usecase/policy"
	policymock "github.com/sergeyslonimsky/elara/internal/usecase/policy/mocks"
)

// mocks bundles the gomock-backed dependencies. The Casbin enforcer is a
// concrete type and is therefore not part of the mocks struct — tests get it
// alongside the service from setupService.
type mocks struct {
	groups *policymock.MockgroupFinder
}

// setupService wires a real bbolt-backed Casbin enforcer + TxManager and the
// policy.Service under test, plus a gomock group finder. Returning the
// enforcer/txm lets tests seed g-rules through real WriteTx writes and assert
// directly against the in-memory cache, matching the integration-style approach
// used elsewhere in the codebase.
func setupService(t *testing.T) (*policy.Service, *casbin.Enforcer, storage.TxManager, *mocks) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "elara.db")

	store, err := bbolt.Open(path)
	require.NoError(t, err)

	t.Cleanup(func() { _ = store.Close() })

	policies := bbolt.NewPolicyRepo(store)

	enforcer, err := casbin.NewEnforcer(policies)
	require.NoError(t, err)

	txm := bbolt.NewTxManager(store.DB())

	ctrl := gomock.NewController(t)
	m := &mocks{groups: policymock.NewMockgroupFinder(ctrl)}

	return policy.New(enforcer, m.groups, txm), enforcer, txm, m
}
