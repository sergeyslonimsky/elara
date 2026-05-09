package namespace_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/usecase/namespace"
	namespace_mock "github.com/sergeyslonimsky/elara/internal/usecase/namespace/mocks"
)

type mocks struct {
	enforcer *namespace_mock.Mockenforcer
	store    *namespace_mock.Mockstore
	notifier *namespace_mock.Mocknotifier
}

func setupService(t *testing.T) (*namespace.Service, mocks, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)

	m := mocks{
		enforcer: namespace_mock.NewMockenforcer(ctrl),
		store:    namespace_mock.NewMockstore(ctrl),
		notifier: namespace_mock.NewMocknotifier(ctrl),
	}
	svc := namespace.New(m.enforcer, m.store, m.notifier)

	return svc, m, ctrl
}
