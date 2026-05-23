package policy_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
	"github.com/sergeyslonimsky/elara/internal/usecase/policy"
	policymock "github.com/sergeyslonimsky/elara/internal/usecase/policy/mocks"
	"github.com/sergeyslonimsky/elara/test/bbolttest"
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

	_, enforcer, txm := bbolttest.OpenStack(t)

	ctrl := gomock.NewController(t)
	m := &mocks{groups: policymock.NewMockgroupFinder(ctrl)}

	return policy.New(enforcer, m.groups, txm), enforcer, txm, m
}
