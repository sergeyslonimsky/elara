package config_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/usecase/config"
	configmock "github.com/sergeyslonimsky/elara/internal/usecase/config/mocks"
)

type mocks struct {
	pdp               *configmock.Mockpdp
	storage           *configmock.Mockstorage
	watcher           *configmock.Mockwatcher
	namespaceProvider *configmock.MocknamespaceProvider
	schemaValidator   *configmock.MockschemaValidator
}

func setupService(t *testing.T) (*config.Service, mocks, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)

	m := mocks{
		pdp:               configmock.NewMockpdp(ctrl),
		storage:           configmock.NewMockstorage(ctrl),
		watcher:           configmock.NewMockwatcher(ctrl),
		namespaceProvider: configmock.NewMocknamespaceProvider(ctrl),
		schemaValidator:   configmock.NewMockschemaValidator(ctrl),
	}
	svc := config.New(m.pdp, m.storage, m.watcher, m.namespaceProvider, m.schemaValidator)

	return svc, m, ctrl
}
