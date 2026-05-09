package policy_test

import (
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/usecase/policy"
	policy_mock "github.com/sergeyslonimsky/elara/internal/usecase/policy/mocks"
)

type mocks struct {
	enforcer *policy_mock.Mockenforcer
	groups   *policy_mock.MockgroupFinder
}

func setupService(ctrl *gomock.Controller) (*policy.Service, mocks) {
	m := mocks{
		enforcer: policy_mock.NewMockenforcer(ctrl),
		groups:   policy_mock.NewMockgroupFinder(ctrl),
	}

	return policy.New(m.enforcer, m.groups), m
}
