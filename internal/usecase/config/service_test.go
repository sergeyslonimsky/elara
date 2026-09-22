package config_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	storage_mock "github.com/sergeyslonimsky/elara/internal/storage/mocks"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
	configmock "github.com/sergeyslonimsky/elara/internal/usecase/config/mocks"
)

const (
	testUserID = "11111111-2222-3333-4444-555555555555"
)

type mocks struct {
	txm               *storage_mock.MockManager
	pdp               *configmock.Mockpdp
	storage           *configmock.MockconfigRepo
	kv                *configmock.MockkvRepo
	watcher           *configmock.Mockwatcher
	namespaceProvider *configmock.MocknamespaceProvider
	schemaValidator   *configmock.MockschemaValidator
}

// repoMock satisfies both configRepo and kvRepo by embedding their separate
// mocks — config.New takes one repo value typed as the union of the two
// (see its doc comment); most tests only ever set expectations on one side.
type repoMock struct {
	*configmock.MockconfigRepo
	*configmock.MockkvRepo
}

func setupService(t *testing.T) (*config.Service, mocks, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)

	m := mocks{
		txm:               storage_mock.NewMockManager(ctrl),
		pdp:               configmock.NewMockpdp(ctrl),
		storage:           configmock.NewMockconfigRepo(ctrl),
		kv:                configmock.NewMockkvRepo(ctrl),
		watcher:           configmock.NewMockwatcher(ctrl),
		namespaceProvider: configmock.NewMocknamespaceProvider(ctrl),
		schemaValidator:   configmock.NewMockschemaValidator(ctrl),
	}
	repo := repoMock{m.storage, m.kv}
	svc := config.New(m.txm, m.pdp, repo, m.watcher, m.namespaceProvider, m.schemaValidator)

	return svc, m, ctrl
}
