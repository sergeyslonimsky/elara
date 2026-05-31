package config_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	storage_mock "github.com/sergeyslonimsky/elara/internal/storage/mocks"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
	configmock "github.com/sergeyslonimsky/elara/internal/usecase/config/mocks"
)

type mocks struct {
	txm               *storage_mock.MockManager
	pdp               *configmock.Mockpdp
	storage           *configmock.MockstorageRepo
	watcher           *configmock.Mockwatcher
	namespaceProvider *configmock.MocknamespaceProvider
	schemaValidator   *configmock.MockschemaValidator
}

func setupService(t *testing.T) (*config.Service, mocks, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)

	m := mocks{
		txm:               storage_mock.NewMockManager(ctrl),
		pdp:               configmock.NewMockpdp(ctrl),
		storage:           configmock.NewMockstorageRepo(ctrl),
		watcher:           configmock.NewMockwatcher(ctrl),
		namespaceProvider: configmock.NewMocknamespaceProvider(ctrl),
		schemaValidator:   configmock.NewMockschemaValidator(ctrl),
	}
	svc := config.New(m.txm, m.pdp, m.storage, m.watcher, m.namespaceProvider, m.schemaValidator)

	return svc, m, ctrl
}
