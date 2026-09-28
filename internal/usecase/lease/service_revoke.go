package lease

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

// Revoke deletes a lease and every key attached to it.
//
// One transaction covers the lease record, both indexes and the keys, so a
// watcher never sees a lease's keys disappear while the lease still exists — or
// the reverse, which would leave keys nothing will ever delete.
func (s *Service) Revoke(ctx context.Context, id int64) error {
	err := s.txm.WithTx(ctx, func(ctx context.Context) error {
		return s.revokeTx(ctx, id)
	})
	if err != nil {
		return fmt.Errorf("revoke lease %d: %w", id, err)
	}

	return nil
}

// revokeTx is the body of a revoke, without its own transaction, so that the
// expiry sweep can reuse it per lease.
func (s *Service) revokeTx(ctx context.Context, id int64) error {
	refs, err := s.leases.Revoke(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrResourceNotFound) {
			return fmt.Errorf("lease %d: %w", id, domain.ErrLeaseNotFound)
		}

		return fmt.Errorf("drop lease: %w", err)
	}

	if len(refs) == 0 {
		return nil
	}

	if _, _, err := s.keys.DeleteKeys(ctx, refs, false); err != nil {
		return fmt.Errorf("delete lease keys: %w", err)
	}

	return nil
}
