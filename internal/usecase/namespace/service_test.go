package namespace_test

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	storage_mock "github.com/sergeyslonimsky/elara/internal/storage/mocks"
	"github.com/sergeyslonimsky/elara/internal/usecase/namespace"
	namespacemock "github.com/sergeyslonimsky/elara/internal/usecase/namespace/mocks"
)

// expectTxPassthrough wires the txm mock so any WithTx call runs its
// callback inline with the same ctx. Use it whenever the production code
// opens a WithTx whose only purpose is to acquire a tx-backed querier
// (e.g. populateConfigCount around configs.CountByNamespace) and the test
// is not asserting on tx semantics. AnyTimes allows nested / repeated
// calls without per-case counting.
func expectTxPassthrough(m mocks) {
	m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	).AnyTimes()
}

const testUserID = "11111111-2222-3333-4444-555555555555"

type mocks struct {
	txm      *storage_mock.MockManager
	pdp      *namespacemock.Mockpdp
	store    *namespacemock.Mockstore
	configs  *namespacemock.MockconfigCounter
	notifier *namespacemock.Mocknotifier
}

func setupService(t *testing.T) (*namespace.Service, mocks, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)

	mock := mocks{
		txm:      storage_mock.NewMockManager(ctrl),
		pdp:      namespacemock.NewMockpdp(ctrl),
		store:    namespacemock.NewMockstore(ctrl),
		configs:  namespacemock.NewMockconfigCounter(ctrl),
		notifier: namespacemock.NewMocknotifier(ctrl),
	}
	svc := namespace.New(mock.txm, mock.pdp, mock.store, mock.configs, mock.notifier)

	return svc, mock, ctrl
}
