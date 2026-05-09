package clients_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/usecase/clients"
	clientsmock "github.com/sergeyslonimsky/elara/internal/usecase/clients/mocks"
)

type mocks struct {
	enforcer *clientsmock.Mockenforcer
	active   *clientsmock.MockactiveSource
	history  *clientsmock.MockhistorySource
}

func setupService(t *testing.T) (*clients.Service, mocks, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)

	m := mocks{
		enforcer: clientsmock.NewMockenforcer(ctrl),
		active:   clientsmock.NewMockactiveSource(ctrl),
		history:  clientsmock.NewMockhistorySource(ctrl),
	}
	svc := clients.New(m.enforcer, m.active, m.history)

	return svc, m, ctrl
}
