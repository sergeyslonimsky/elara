package auth_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	authmock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

type mocks struct {
	provider *authmock.MockoidcProvider
	users    *authmock.MockuserStore
	session  *authmock.MocksessionCreator
	admin    *authmock.MockadminBootstrap
}

func setupService(t *testing.T, ctrl *gomock.Controller) (*authuc.Service, mocks) {
	t.Helper()

	m := mocks{
		provider: authmock.NewMockoidcProvider(ctrl),
		users:    authmock.NewMockuserStore(ctrl),
		session:  authmock.NewMocksessionCreator(ctrl),
		admin:    authmock.NewMockadminBootstrap(ctrl),
	}

	return authuc.New(m.provider, m.users, m.session, m.admin, "admin@example.com"), m
}
