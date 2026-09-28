package lease

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

// grantIDAttempts caps how many times a generated ID is redrawn after a
// collision. Hitting even the second attempt on a 63-bit random space means
// something is wrong with the source of randomness, not that we were unlucky.
const grantIDAttempts = 3

// Grant creates a lease running for ttl.
//
// id zero asks the server to pick one; any other value is honoured, which etcd
// permits and clients occasionally rely on to make a lease reproducible. A
// caller-supplied ID that is taken is an error rather than a silent reuse — two
// clients sharing a lease would expire each other's keys.
func (s *Service) Grant(ctx context.Context, id int64, ttl time.Duration) (*domain.Lease, error) {
	ttl, err := s.clampTTL(ttl)
	if err != nil {
		return nil, err
	}

	if id != 0 {
		return s.grantWithID(ctx, id, ttl)
	}

	for range grantIDAttempts {
		generated, err := domain.NewLeaseID()
		if err != nil {
			return nil, fmt.Errorf("grant lease: %w", err)
		}

		l, err := s.grantWithID(ctx, generated, ttl)
		if err == nil {
			return l, nil
		}

		if !errors.Is(err, domain.ErrLeaseExists) {
			return nil, err
		}
	}

	return nil, fmt.Errorf("grant lease: %w", domain.ErrLeaseIDExhausted)
}

// grantWithID writes one lease, translating the storage-level collision into a
// domain error the caller can branch on.
func (s *Service) grantWithID(ctx context.Context, id int64, ttl time.Duration) (*domain.Lease, error) {
	l, err := domain.NewLease(id, ttl, time.Now())
	if err != nil {
		return nil, fmt.Errorf("grant lease: %w", err)
	}

	err = s.txm.WithTx(ctx, func(ctx context.Context) error {
		if err := s.leases.Grant(ctx, l); err != nil {
			return fmt.Errorf("store lease: %w", err)
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, storage.ErrResourceAlreadyExists) {
			return nil, fmt.Errorf("lease %d: %w", id, domain.ErrLeaseExists)
		}

		return nil, fmt.Errorf("grant lease tx: %w", err)
	}

	return l, nil
}

// clampTTL applies the deployment's bounds to a requested TTL.
//
// Clamping rather than rejecting, because that is what etcd does with a TTL
// below its own minimum: the client is told the granted TTL in the response and
// paces its keepalives by that, so a silently shortened lease still behaves
// correctly. A non-positive TTL is a different matter and is rejected by
// domain.NewLease.
func (s *Service) clampTTL(ttl time.Duration) (time.Duration, error) {
	if ttl <= 0 {
		return 0, fmt.Errorf("lease ttl %s: %w", ttl, domain.ErrLeaseTTLInvalid)
	}

	if s.cfg.MinTTL > 0 && ttl < s.cfg.MinTTL {
		return s.cfg.MinTTL, nil
	}

	if s.cfg.MaxTTL > 0 && ttl > s.cfg.MaxTTL {
		return s.cfg.MaxTTL, nil
	}

	return ttl, nil
}
