package namespace_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/usecase/namespace"
	namespacemock "github.com/sergeyslonimsky/elara/internal/usecase/namespace/mocks"
)

type mocks struct {
	pdp      *namespacemock.Mockpdp
	store    *namespacemock.Mockstore
	notifier *namespacemock.Mocknotifier
}

func setupService(t *testing.T) (*namespace.Service, mocks, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)

	m := mocks{
		pdp:      namespacemock.NewMockpdp(ctrl),
		store:    namespacemock.NewMockstore(ctrl),
		notifier: namespacemock.NewMocknotifier(ctrl),
	}
	svc := namespace.New(m.pdp, m.store, m.notifier)

	return svc, m, ctrl
}
