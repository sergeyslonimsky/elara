package config_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/usecase/config"
	config_mock "github.com/sergeyslonimsky/elara/internal/usecase/config/mocks"
)

type mocks struct {
	enforcer          *config_mock.Mockenforcer
	storage           *config_mock.Mockstorage
	watcher           *config_mock.Mockwatcher
	namespaceProvider *config_mock.MocknamespaceProvider
	schemaValidator   *config_mock.MockschemaValidator
}

func setupService(t *testing.T) (*config.Service, mocks, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)

	m := mocks{
		enforcer:          config_mock.NewMockenforcer(ctrl),
		storage:           config_mock.NewMockstorage(ctrl),
		watcher:           config_mock.NewMockwatcher(ctrl),
		namespaceProvider: config_mock.NewMocknamespaceProvider(ctrl),
		schemaValidator:   config_mock.NewMockschemaValidator(ctrl),
	}
	svc := config.New(m.enforcer, m.storage, m.watcher, m.namespaceProvider, m.schemaValidator)

	return svc, m, ctrl
}
