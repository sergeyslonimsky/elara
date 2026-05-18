package dashboard_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/usecase/dashboard"
	dashboardmock "github.com/sergeyslonimsky/elara/internal/usecase/dashboard/mocks"
)

type mocks struct {
	enforcer      *dashboardmock.Mockenforcer
	namespaces    *dashboardmock.MocknsLister
	configs       *dashboardmock.MockconfigCounter
	activity      *dashboardmock.MockactivitySource
	activeClients *dashboardmock.MockactiveClientsSource
}

func setupService(t *testing.T) (*dashboard.Service, mocks, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)

	m := mocks{
		enforcer:      dashboardmock.NewMockenforcer(ctrl),
		namespaces:    dashboardmock.NewMocknsLister(ctrl),
		configs:       dashboardmock.NewMockconfigCounter(ctrl),
		activity:      dashboardmock.NewMockactivitySource(ctrl),
		activeClients: dashboardmock.NewMockactiveClientsSource(ctrl),
	}
	svc := dashboard.New(m.enforcer, m.namespaces, m.configs, m.activity, m.activeClients)

	return svc, m, ctrl
}
