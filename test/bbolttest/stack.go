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
)

// OpenStack opens a fresh bbolt database under t.TempDir, wires a Casbin
// enforcer over its PolicyRepo, and constructs a TxManager bound to the
// same DB. The store is closed in t.Cleanup.
func OpenStack(t *testing.T) (*bbolt.Store, *casbin.Enforcer, *bbolt.Manager) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "elara.db")

	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	storageManager := bbolt.NewManager(store.DB())
	policies := bbolt.NewPolicyRepo(storageManager)

	enforcer, err := casbin.NewEnforcer(policies)
	require.NoError(t, err)

	return store, enforcer, storageManager
}
