package dashboard_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/usecase/dashboard"
	dashboard_mock "github.com/sergeyslonimsky/elara/internal/usecase/dashboard/mocks"
)

type mocks struct {
	enforcer      *dashboard_mock.Mockenforcer
	namespaces    *dashboard_mock.MocknsLister
	configs       *dashboard_mock.MockconfigCounter
	activity      *dashboard_mock.MockactivitySource
	activeClients *dashboard_mock.MockactiveClientsSource
}

func setupService(t *testing.T) (*dashboard.Service, mocks, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)

	m := mocks{
		enforcer:      dashboard_mock.NewMockenforcer(ctrl),
		namespaces:    dashboard_mock.NewMocknsLister(ctrl),
		configs:       dashboard_mock.NewMockconfigCounter(ctrl),
		activity:      dashboard_mock.NewMockactivitySource(ctrl),
		activeClients: dashboard_mock.NewMockactiveClientsSource(ctrl),
	}
	svc := dashboard.New(m.enforcer, m.namespaces, m.configs, m.activity, m.activeClients)

	return svc, m, ctrl
}
