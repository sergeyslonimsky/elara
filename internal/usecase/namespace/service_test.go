package namespace_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	storage_mock "github.com/sergeyslonimsky/elara/internal/storage/mocks"
	"github.com/sergeyslonimsky/elara/internal/usecase/namespace"
	namespacemock "github.com/sergeyslonimsky/elara/internal/usecase/namespace/mocks"
)

const testUserID = "11111111-2222-3333-4444-555555555555"

type mocks struct {
	txm      *storage_mock.MockManager
	pdp      *namespacemock.Mockpdp
	store    *namespacemock.Mockstore
	notifier *namespacemock.Mocknotifier
}

func setupService(t *testing.T) (*namespace.Service, mocks, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)

	mock := mocks{
		txm:      storage_mock.NewMockManager(ctrl),
		pdp:      namespacemock.NewMockpdp(ctrl),
		store:    namespacemock.NewMockstore(ctrl),
		notifier: namespacemock.NewMocknotifier(ctrl),
	}
	svc := namespace.New(mock.txm, mock.pdp, mock.store, mock.notifier)

	return svc, mock, ctrl
}
