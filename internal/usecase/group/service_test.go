package group_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
	"github.com/sergeyslonimsky/elara/internal/usecase/group"
	"github.com/sergeyslonimsky/elara/test/bbolttest"
)

// testStack wires the dependencies the table-driven group tests need so each
// case can drive setup with a single value. Tests use the real bbolt +
// Casbin stack rather than mocks because the service depends on concrete
// per-tx views whose return types do not fit cleanly into mockable
// interfaces; the integration helper also exercises the §4 level-2
// atomicity invariant end-to-end.
type testStack struct {
	svc      *group.Service
	store    *bbolt.Store
	enforcer *casbin.Enforcer
	repo     *bbolt.GroupRepo
	txm      *bbolt.TxManager
}

func newTestStack(t *testing.T) testStack {
	t.Helper()

	store, enforcer, txm := bbolttest.OpenStack(t)
	repo := bbolt.NewGroupRepo(store)
	pdp := authz.NewPDP(enforcer)
	pap := authz.NewPAP(enforcer, txm)
	scope := authz.NewScope(pdp, pap, repo)

	return testStack{
		svc:      group.New(repo, pdp, pap, scope),
		store:    store,
		enforcer: enforcer,
		repo:     repo,
		txm:      txm,
	}
}

func adminAuth() domain.AuthInfo {
	return domain.AuthInfo{Email: "admin@example.com"}
}

// int64Ptr is a single-call helper for assembling optional optimistic-lock
// fields in test data without scattering temporary variables across cases.
func int64Ptr(v int64) *int64 { return &v }

// seedAdminWildcard grants admin@example.com the (*,*,*) policy used by
// happy-path cases where the PDP must accept any permission delta.
func seedAdminWildcard(t *testing.T, st testStack) {
	t.Helper()

	require.NoError(t, st.enforcer.WriteTx(t.Context(), st.txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		return txe.AddPolicy("admin@example.com", domain.DomainAll, domain.ObjectAll, domain.ActionAll)
	}))
}
