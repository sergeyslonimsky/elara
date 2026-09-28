package lease

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

// TimeToLive returns a lease and, when withKeys is set, the keys attached to it.
//
// An expired lease that the sweep has not reached yet is still returned: the
// entity knows it has nothing left (domain.Lease.RemainingTTL), and reporting
// "not found" for a record that exists would make the answer depend on sweep
// timing rather than on state.
func (s *Service) TimeToLive(
	ctx context.Context,
	id int64,
	withKeys bool,
) (*domain.Lease, []domain.KeyRef, error) {
	l, err := s.load(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	if !withKeys {
		return l, nil, nil
	}

	refs, err := s.leases.KeysOf(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("lease keys: %w", err)
	}

	return l, refs, nil
}

// List returns every lease the store holds.
func (s *Service) List(ctx context.Context) ([]*domain.Lease, error) {
	leases, err := s.leases.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list leases: %w", err)
	}

	return leases, nil
}

// load fetches a lease, translating the storage-level miss into the domain error
// every caller in this package reports.
func (s *Service) load(ctx context.Context, id int64) (*domain.Lease, error) {
	l, err := s.leases.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrResourceNotFound) {
			return nil, fmt.Errorf("lease %d: %w", id, domain.ErrLeaseNotFound)
		}

		return nil, fmt.Errorf("get lease: %w", err)
	}

	return l, nil
}
