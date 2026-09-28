// Package lease is the usecase layer for etcd lease grants: the TTL objects
// clients attach keys to in order to build distributed locks and leader
// election.
//
// It owns the transaction boundary for every lease operation. The repositories
// it calls join the transaction from context and never open their own, which is
// what lets a revoke delete a lease, its index entries and its keys as one unit.
package lease

import (
	"context"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=lease_mock -source=service.go

type (
	// leaseRepo is lease state: the records and the two indexes over them.
	leaseRepo interface {
		Grant(ctx context.Context, l *domain.Lease) error
		Get(ctx context.Context, id int64) (*domain.Lease, error)
		Renew(ctx context.Context, id int64, expiresAt time.Time) error
		Revoke(ctx context.Context, id int64) ([]domain.KeyRef, error)
		KeysOf(ctx context.Context, id int64) ([]domain.KeyRef, error)
		List(ctx context.Context) ([]*domain.Lease, error)
		ExpiredBefore(ctx context.Context, t time.Time, limit int) ([]int64, error)
	}

	// keyDeleter removes the keys a revoked lease held.
	//
	// Deliberately the config usecase rather than its repository: deleting keys
	// has to allocate one revision for the whole set and publish the watch events
	// after the commit, and both of those live on the write path that already
	// owns them (see usecase/config.Service.DeleteKeys).
	keyDeleter interface {
		DeleteKeys(
			ctx context.Context,
			refs []domain.KeyRef,
			returnPrev bool,
		) ([]*domain.KVPair, int64, error)
	}
)

// Config bounds what a client may ask for, and how often a renewal reaches the
// store.
type Config struct {
	// MinTTL and MaxTTL clamp a requested grant. Zero means unbounded.
	MinTTL time.Duration
	MaxTTL time.Duration

	// CheckpointThreshold is the drift a renewal may accumulate before it is
	// persisted, as a fraction of the TTL. See domain.Lease.NeedsCheckpoint.
	CheckpointThreshold float64
}

type Service struct {
	txm    storage.Manager
	leases leaseRepo
	keys   keyDeleter
	cfg    Config
}

func New(txm storage.Manager, leases leaseRepo, keys keyDeleter, cfg Config) *Service {
	return &Service{txm: txm, leases: leases, keys: keys, cfg: cfg}
}
