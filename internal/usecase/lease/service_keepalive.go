package lease

import (
	"context"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// KeepAlive extends a lease by a full TTL from now and returns it with the
// extension applied.
//
// The returned lease is what the client is promised; the store may hold an older
// expiry. Persisting every renewal would mean a write transaction on every ping —
// KeepAlive arrives roughly every TTL/3 per session — so the write happens only
// once the drift is worth it (domain.Lease.NeedsCheckpoint). The gap is bounded
// by the threshold, and the cost is that an abrupt restart can expire a lease up
// to that much earlier than promised.
//
// Renewing an already-expired lease is refused rather than quietly resurrecting
// it: the expiry sweep may have deleted its keys already, and a lease whose keys
// are gone is not the lease the client thinks it is holding.
func (s *Service) KeepAlive(ctx context.Context, id int64) (*domain.Lease, error) {
	l, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if err := l.EnsureLive(now); err != nil {
		return nil, fmt.Errorf("keepalive lease %d: %w", id, err)
	}

	renewed := l.RenewedExpiry(now)

	if l.NeedsCheckpoint(renewed, s.cfg.CheckpointThreshold) {
		err := s.txm.WithTx(ctx, func(ctx context.Context) error {
			if err := s.leases.Renew(ctx, id, renewed); err != nil {
				return fmt.Errorf("renew lease: %w", err)
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("keepalive lease tx: %w", err)
		}
	}

	extended := *l
	extended.ExpiresAt = renewed

	return &extended, nil
}
