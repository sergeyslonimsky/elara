package user_test

import (
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/usecase/user"
	user_mock "github.com/sergeyslonimsky/elara/internal/usecase/user/mocks"
)

type mocks struct {
	enforcer *user_mock.Mockenforcer
	store    *user_mock.Mockstore
}

func setupService(ctrl *gomock.Controller) (*user.Service, mocks) {
	m := mocks{
		enforcer: user_mock.NewMockenforcer(ctrl),
		store:    user_mock.NewMockstore(ctrl),
	}

	return user.New(m.enforcer, m.store), m
}
