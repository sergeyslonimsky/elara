package authz_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	policyrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/policy"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

// policyBucketName mirrors internal/storage/bbolt/policy.bucketPolicy (private
// to that package) so tests can force I/O errors on demand — see
// breakPolicyBucket.
const policyBucketName = "auth_policy"

// newTestPAP builds a PAP backed by a real bbolt-backed Casbin enforcer in a
// temp directory. PAP/PAPTx wrap concrete *casbin.Enforcer /
// *casbin.TxEnforcer types (not interfaces), so exercising them requires a
// real enforcer rather than a gomock double — mirrors the pattern used by
// internal/service/auth/casbin/enforcer_test.go.
func newTestPAP(t *testing.T) (*authz.PAP, storage.Manager, *bolt.DB) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "casbin.db")

	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	db := store.DB()
	txm := bbolt.NewManager(db)
	policies := policyrepo.NewRepository(pkgbbolt.NewManager(db))

	e, err := casbin.NewEnforcer(policies)
	require.NoError(t, err)

	return authz.NewPAP(e, txm), txm, db
}

// breakPolicyBucket drops the auth_policy bucket so any subsequent
// PolicyRepository call (Add/Remove/List) fails with a wrapped
// ErrBucketNotFound, letting tests exercise PAP/PAPTx error-passthrough
// branches without a mock (PolicyRepository has no generated mock and lives
// in a different package).
func breakPolicyBucket(t *testing.T, db *bolt.DB) {
	t.Helper()

	require.NoError(t, db.Update(func(tx *bolt.Tx) error {
		return tx.DeleteBucket([]byte(policyBucketName))
	}))
}
