package auth_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	storage_mock "github.com/sergeyslonimsky/elara/internal/storage/mocks"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	authmock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

type mocks struct {
	txm      *storage_mock.MockManager
	provider *authmock.MockoidcProvider
	users    *authmock.MockuserStore
	admin    *authmock.MockadminBootstrap
	sessions *authmock.MocksessionsService
}

func setupService(t *testing.T, ctrl *gomock.Controller) (*authuc.Service, mocks) {
	t.Helper()

	m := mocks{
		txm:      storage_mock.NewMockManager(ctrl),
		provider: authmock.NewMockoidcProvider(ctrl),
		users:    authmock.NewMockuserStore(ctrl),
		admin:    authmock.NewMockadminBootstrap(ctrl),
		sessions: authmock.NewMocksessionsService(ctrl),
	}

	return authuc.New(m.txm, m.provider, m.users, m.admin, m.sessions, "admin@example.com"), m
}
