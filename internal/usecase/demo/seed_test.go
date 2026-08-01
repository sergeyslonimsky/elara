package demo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	storage_mock "github.com/sergeyslonimsky/elara/internal/storage/mocks"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
	configmock "github.com/sergeyslonimsky/elara/internal/usecase/config/mocks"
	"github.com/sergeyslonimsky/elara/internal/usecase/demo"
	"github.com/sergeyslonimsky/elara/internal/usecase/namespace"
	namespacemock "github.com/sergeyslonimsky/elara/internal/usecase/namespace/mocks"
	"github.com/sergeyslonimsky/elara/internal/usecase/schema"
	schemamock "github.com/sergeyslonimsky/elara/internal/usecase/schema/mocks"
)

// countingRegistry is a capturing double for the package-private clientRegistry
// interface consumed by Seed. It only tracks how many times each method was hit
// so the Seed tests can assert simulated clients were (re)injected.
type countingRegistry struct {
	nextID      int
	connections int
	watches     int
	requests    int
}

func (r *countingRegistry) RegisterConnection(_ domain.ConnectionInfo) string {
	r.nextID++
	r.connections++

	return "conn"
}

func (r *countingRegistry) RegisterWatch(_ string, _ domain.ActiveWatch) {
	r.watches++
}

func (r *countingRegistry) RecordRequest(
	_, _, _ string,
	_ int64,
	_ time.Duration,
	_ error,
) {
	r.requests++
}

type nsMocks struct {
	txm      *storage_mock.MockManager
	store    *namespacemock.Mockstore
	configs  *namespacemock.MockconfigCounter
	notifier *namespacemock.Mocknotifier
	pdp      *namespacemock.Mockpdp
}

type cfgMocks struct {
	txm               *storage_mock.MockManager
	storage           *configmock.MockstorageRepo
	watcher           *configmock.Mockwatcher
	namespaceProvider *configmock.MocknamespaceProvider
	schemaValidator   *configmock.MockschemaValidator
	pdp               *configmock.Mockpdp
}

type schemaMocks struct {
	store      *schemamock.Mockstore
	nsProvider *schemamock.MocknsProvider
	pdp        *schemamock.Mockpdp
}

type demoMocks struct {
	ns       nsMocks
	cfg      cfgMocks
	schema   schemaMocks
	registry *countingRegistry
}

func setup(t *testing.T) (demo.Deps, demoMocks) {
	t.Helper()
	ctrl := gomock.NewController(t)

	m := demoMocks{
		ns: nsMocks{
			txm:      storage_mock.NewMockManager(ctrl),
			store:    namespacemock.NewMockstore(ctrl),
			configs:  namespacemock.NewMockconfigCounter(ctrl),
			notifier: namespacemock.NewMocknotifier(ctrl),
			pdp:      namespacemock.NewMockpdp(ctrl),
		},
		cfg: cfgMocks{
			txm:               storage_mock.NewMockManager(ctrl),
			storage:           configmock.NewMockstorageRepo(ctrl),
			watcher:           configmock.NewMockwatcher(ctrl),
			namespaceProvider: configmock.NewMocknamespaceProvider(ctrl),
			schemaValidator:   configmock.NewMockschemaValidator(ctrl),
			pdp:               configmock.NewMockpdp(ctrl),
		},
		schema: schemaMocks{
			store:      schemamock.NewMockstore(ctrl),
			nsProvider: schemamock.NewMocknsProvider(ctrl),
			pdp:        schemamock.NewMockpdp(ctrl),
		},
		registry: &countingRegistry{},
	}

	deps := demo.Deps{
		Namespaces: namespace.New(m.ns.txm, m.ns.pdp, m.ns.store, m.ns.configs, m.ns.notifier),
		Configs: config.New(
			m.cfg.txm,
			m.cfg.pdp,
			m.cfg.storage,
			m.cfg.watcher,
			m.cfg.namespaceProvider,
			m.cfg.schemaValidator,
		),
		Schemas: schema.New(m.schema.pdp, m.schema.store, m.schema.nsProvider),
		Clients: m.registry,
	}

	return deps, m
}

// txPassthrough runs any WithTx callback inline with the same ctx.
func txPassthrough(txm *storage_mock.MockManager) {
	txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	).AnyTimes()
}

// notSeeded makes the production-namespace lookup report a fresh instance.
func notSeeded(m demoMocks) {
	m.ns.store.EXPECT().
		Get(gomock.Any(), "production").
		Return(nil, storage.ErrResourceNotFound)
}

// namespacesSucceed lets every namespace Create through. Declared after
// notSeeded so the first (production) Get is matched by notSeeded's exhausted
// expectation and later Gets fall through to this one.
func namespacesSucceed(m demoMocks) {
	txPassthrough(m.ns.txm)
	m.ns.store.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	m.ns.store.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, name string) (*domain.Namespace, error) {
			return &domain.Namespace{Name: name}, nil
		},
	).AnyTimes()
}

// schemasSucceed lets every schema Attach through.
func schemasSucceed(m demoMocks) {
	m.schema.nsProvider.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, name string) (*domain.Namespace, error) {
			return &domain.Namespace{Name: name}, nil
		},
	).AnyTimes()
	m.schema.store.EXPECT().Attach(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
}

// configsSucceed lets every config Create through.
func configsSucceed(m demoMocks) {
	txPassthrough(m.cfg.txm)
	m.cfg.namespaceProvider.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, name string) (*domain.Namespace, error) {
			return &domain.Namespace{Name: name}, nil
		},
	).AnyTimes()
	m.cfg.schemaValidator.EXPECT().
		Validate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()
	m.cfg.storage.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	m.cfg.namespaceProvider.EXPECT().
		UpdateTimestamp(gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()
	m.cfg.watcher.EXPECT().NotifyCreated(gomock.Any(), gomock.Any()).AnyTimes()
}

func TestSeed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(m demoMocks)
		wantErr  string
	}{
		{
			name: "not yet seeded seeds everything and injects clients",
			mockFunc: func(m demoMocks) {
				notSeeded(m)
				namespacesSucceed(m)
				schemasSucceed(m)
				configsSucceed(m)
			},
		},
		{
			name: "already seeded skips persistent seed but still injects clients",
			mockFunc: func(m demoMocks) {
				// production namespace exists -> alreadySeeded == true.
				txPassthrough(m.ns.txm)
				m.ns.store.EXPECT().
					Get(gomock.Any(), "production").
					Return(&domain.Namespace{Name: "production"}, nil)
				m.ns.configs.EXPECT().
					CountByNamespace(gomock.Any(), "production").
					Return(3, nil)
			},
		},
		{
			name: "check seed state error is wrapped",
			mockFunc: func(m demoMocks) {
				m.ns.store.EXPECT().
					Get(gomock.Any(), "production").
					Return(nil, errors.New("boom"))
			},
			wantErr: "check seed state: get \"production\" namespace: get namespace: boom",
		},
		{
			name: "namespace creation failure aborts seed",
			mockFunc: func(m demoMocks) {
				notSeeded(m)
				txPassthrough(m.ns.txm)
				m.ns.store.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(errors.New("db down"))
			},
			wantErr: `create namespace "production"`,
		},
		{
			name: "schema attach failure aborts seed",
			mockFunc: func(m demoMocks) {
				notSeeded(m)
				namespacesSucceed(m)
				m.schema.nsProvider.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Return(&domain.Namespace{Name: "production"}, nil).
					AnyTimes()
				m.schema.store.EXPECT().
					Attach(gomock.Any(), gomock.Any()).
					Return(errors.New("attach boom"))
			},
			wantErr: "attach schema",
		},
		{
			name: "config create failure aborts seed",
			mockFunc: func(m demoMocks) {
				notSeeded(m)
				namespacesSucceed(m)
				schemasSucceed(m)
				txPassthrough(m.cfg.txm)
				m.cfg.namespaceProvider.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, name string) (*domain.Namespace, error) {
						return &domain.Namespace{Name: name}, nil
					}).
					AnyTimes()
				m.cfg.schemaValidator.EXPECT().
					Validate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).
					AnyTimes()
				m.cfg.storage.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(errors.New("write boom"))
			},
			wantErr: "create config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps, m := setup(t)
			tt.mockFunc(m)

			err := demo.Seed(t.Context(), deps)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				// Client injection happens only after persistent seeding succeeds.
				assert.Zero(t, m.registry.connections)

				return
			}

			require.NoError(t, err)
			// Clients are injected on every successful call regardless of
			// whether persistent data was (re)seeded: one connection and one
			// watch per simulated client, plus at least one recorded request.
			assert.Positive(t, m.registry.connections)
			assert.Equal(t, m.registry.connections, m.registry.watches)
			assert.Positive(t, m.registry.requests)
		})
	}
}
