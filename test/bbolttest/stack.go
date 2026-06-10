// Package bbolttest provides a shared bootstrap for usecase tests that need
// a real bbolt store backing a Casbin enforcer and TxManager. It is the
// minimal stack on which every usecase package layers its own repos, PDP,
// and gomock fixtures.
package bbolttest

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	policyrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/policy"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

// Stack bundles the shared bbolt + Casbin fixtures used by usecase tests.
// Txm is the legacy *bbolt.Manager (still required by un-migrated repos);
// PkgManager is the pkg/bbolt.Manager required by migrated repos
// (token/, session/, namespace/, ...). Both point at the same underlying DB.
type Stack struct {
	Store      *bbolt.Store
	Enforcer   *casbin.Enforcer
	Txm        *bbolt.Manager
	PkgManager pkgbbolt.Manager
}

// OpenStack opens a fresh bbolt database under t.TempDir, wires a Casbin
// enforcer over its PolicyRepo, and constructs both manager flavors bound
// to the same DB. The store is closed in t.Cleanup.
func OpenStack(t *testing.T) Stack {
	t.Helper()

	path := filepath.Join(t.TempDir(), "elara.db")

	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	storageManager := bbolt.NewManager(store.DB())
	pkgManager := pkgbbolt.NewManager(store.DB())
	policies := policyrepo.NewRepository(pkgManager)

	enforcer, err := casbin.NewEnforcer(policies)
	require.NoError(t, err)

	return Stack{
		Store:      store,
		Enforcer:   enforcer,
		Txm:        storageManager,
		PkgManager: pkgManager,
	}
}
