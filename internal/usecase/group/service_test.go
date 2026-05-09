package group_test

import (
	"context"

	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/usecase/group"
	group_mock "github.com/sergeyslonimsky/elara/internal/usecase/group/mocks"
)

type mocks struct {
	enforcer     *group_mock.Mockenforcer
	syncEnforcer *group_mock.MocksyncEnforcer
	store        *group_mock.Mockstore
}

func setupService(ctrl *gomock.Controller) (*group.Service, mocks) {
	m := mocks{
		enforcer:     group_mock.NewMockenforcer(ctrl),
		syncEnforcer: group_mock.NewMocksyncEnforcer(ctrl),
		store:        group_mock.NewMockstore(ctrl),
	}

	return group.New(m.enforcer, m.syncEnforcer, m.store), m
}

func mockFuncWithContext(
	f func(ctx context.Context, m mocks) context.Context,
) func(context.Context, *gomock.Controller) (*group.Service, context.Context) {
	return func(ctx context.Context, ctrl *gomock.Controller) (*group.Service, context.Context) {
		sut, m := setupService(ctrl)
		ctx = f(ctx, m)

		return sut, ctx
	}
}
