package auth_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

type mocks struct {
	provider *auth_mock.MockoidcProvider
	users    *auth_mock.MockuserStore
	session  *auth_mock.MocksessionCreator
	enforcer *auth_mock.MockbootstrapEnforcer
}

func setupService(t *testing.T, ctrl *gomock.Controller) (*authuc.Service, mocks) {
	t.Helper()

	m := mocks{
		provider: auth_mock.NewMockoidcProvider(ctrl),
		users:    auth_mock.NewMockuserStore(ctrl),
		session:  auth_mock.NewMocksessionCreator(ctrl),
		enforcer: auth_mock.NewMockbootstrapEnforcer(ctrl),
	}

	return authuc.New(m.provider, m.users, m.session, m.enforcer, "admin@example.com"), m
}
