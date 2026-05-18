package user_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/usecase/user"
	usermock "github.com/sergeyslonimsky/elara/internal/usecase/user/mocks"
)

type mocks struct {
	store *usermock.Mockstore
}

// setupService wires a Service with a mocked store and real (bbolt-backed)
// Enforcer + UserRepo + TxManager. Paths that go through Enforcer.WriteTx
// (Delete) exercise the integration helper end-to-end; pure store paths
// (Create, Get, List, ResetPassword) drive the mocked store.
func setupService(t *testing.T) (*user.Service, mocks, *gomock.Controller) {
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

	m := mocks{store: usermock.NewMockstore(ctrl)}

	return user.New(enforcer, m.store, users, txm), m, ctrl
}
