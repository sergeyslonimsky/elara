package clients_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/usecase/clients"
	clientsmock "github.com/sergeyslonimsky/elara/internal/usecase/clients/mocks"
)

const (
	testUserID    = "11111111-2222-3333-4444-555555555555"
	testUserEmail = "test@example.com"
)

type mocks struct {
	pdp     *clientsmock.Mockpdp
	active  *clientsmock.MockactiveSource
	history *clientsmock.MockhistorySource
}

func setupService(t *testing.T) (*clients.Service, mocks, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)

	m := mocks{
		pdp:     clientsmock.NewMockpdp(ctrl),
		active:  clientsmock.NewMockactiveSource(ctrl),
		history: clientsmock.NewMockhistorySource(ctrl),
	}
	svc := clients.New(m.pdp, m.active, m.history)

	return svc, m, ctrl
}
