package lease_test

import (
	"context"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	storage_mock "github.com/sergeyslonimsky/elara/internal/storage/mocks"
	"github.com/sergeyslonimsky/elara/internal/usecase/lease"
	lease_mock "github.com/sergeyslonimsky/elara/internal/usecase/lease/mocks"
)

type mocks struct {
	txm    *storage_mock.MockManager
	leases *lease_mock.MockleaseRepo
	keys   *lease_mock.MockkeyDeleter
}

func setupService(t *testing.T, cfg lease.Config) (*lease.Service, mocks) {
	t.Helper()

	ctrl := gomock.NewController(t)
	m := mocks{
		txm:    storage_mock.NewMockManager(ctrl),
		leases: lease_mock.NewMockleaseRepo(ctrl),
		keys:   lease_mock.NewMockkeyDeleter(ctrl),
	}

	return lease.New(m.txm, m.leases, m.keys, cfg), m
}

// expectPassthroughTx runs the transaction callback with the context it was
// given. times is explicit because several flows here are deliberately one
// transaction per lease rather than one for the batch.
func expectPassthroughTx(m mocks, times int) {
	m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).Times(times).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)
}

func liveLease(id int64, ttl, remaining time.Duration) *domain.Lease {
	now := time.Now()

	return &domain.Lease{
		ID:        id,
		TTL:       ttl,
		GrantedAt: now.Add(remaining - ttl),
		ExpiresAt: now.Add(remaining),
	}
}
