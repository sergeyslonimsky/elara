package group_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	grouprepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/group"
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
	repo     *grouprepo.Repository
	txm      *bbolt.Manager
}

func newTestStack(t *testing.T) testStack {
	t.Helper()

	stack := bbolttest.OpenStack(t)
	txm := stack.Txm
	repo := grouprepo.NewRepository(stack.PkgManager)
	pdp := authz.NewPDP(stack.Enforcer)
	pap := authz.NewPAP(stack.Enforcer, txm)
	scope := authz.NewScope(pdp, pap, repo)

	return testStack{
		svc:      group.New(txm, repo, pdp, pap, scope),
		store:    stack.Store,
		enforcer: stack.Enforcer,
		repo:     repo,
		txm:      txm,
	}
}

// adminID is the stable Casbin subject used as the admin's UserID in tests.
// Production code resolves actor.UserID as the Casbin subject, so both
// seedAdminWildcard and adminAuth must use the same value.
const adminID = "admin-id"

func adminAuth() domain.AuthInfo {
	return domain.AuthInfo{UserID: adminID, Email: "admin@example.com"}
}

// seedAdminWildcard grants adminID the (*,*,*) policy used by
// happy-path cases where the PDP must accept any permission delta.
func seedAdminWildcard(t *testing.T, st testStack) {
	t.Helper()

	require.NoError(
		t,
		st.enforcer.WriteTx(t.Context(), st.txm, func(ctx context.Context, txe *casbin.TxEnforcer) error {
			return txe.AddPolicy(
				adminID,
				domain.DomainAll,
				string(domain.ObjectAll),
				string(domain.ActionAll),
			)
		}),
	)
}
