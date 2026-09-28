package lease

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/usecase/txevents"
)

// Revoke deletes a lease and every key attached to it.
//
// One transaction covers the lease record, both indexes and the keys, so a
// watcher never sees a lease's keys disappear while the lease still exists — or
// the reverse, which would leave keys nothing will ever delete.
//
// The notification collector is installed here, around the transaction, not
// left to the config write path inside it. Whoever opens the outermost
// transaction has to own publication: the inner write would otherwise flush as
// soon as its own (flattened) call returned, which is before this transaction
// commits — and a watcher told of a delete it cannot yet read goes on waiting
// for an event that has already been sent.
func (s *Service) Revoke(ctx context.Context, id int64) error {
	outer, pending, owner := txevents.Install(ctx)

	err := s.txm.WithTx(outer, func(ctx context.Context) error {
		return s.revokeTx(ctx, id)
	})
	if err != nil {
		return fmt.Errorf("revoke lease %d: %w", id, err)
	}

	if owner {
		pending.Flush(ctx)
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
