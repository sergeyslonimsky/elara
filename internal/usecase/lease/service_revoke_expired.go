package lease

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// RevokeExpired revokes the leases that were due before now, at most limit of
// them, and reports how many were revoked.
//
// now is a parameter rather than a call to time.Now inside, because the decision
// "this lease has expired" is made from stored state and an explicit instant.
// That is what keeps the operation replayable: once the store replicates, the
// same sweep applied on another node reaches the same conclusion instead of
// depending on when that node happened to tick.
//
// Each lease gets its own transaction, deliberately. One lease that cannot be
// revoked — a namespace an operator locked, say — must not hold up the rest, and
// must not roll back the ones already done. Its error is returned alongside the
// count so the caller can log it; the next sweep will try that lease again,
// which is safe because revoking is idempotent.
func (s *Service) RevokeExpired(ctx context.Context, now time.Time, limit int) (int, error) {
	ids, err := s.leases.ExpiredBefore(ctx, now, limit)
	if err != nil {
		return 0, fmt.Errorf("list expired leases: %w", err)
	}

	var (
		revoked int
		failed  []error
	)

	for _, id := range ids {
		err := s.txm.WithTx(ctx, func(ctx context.Context) error {
			return s.revokeTx(ctx, id)
		})

		switch {
		case err == nil:
			revoked++
		case errors.Is(err, domain.ErrLeaseNotFound):
			// Already gone — a client revoked it between the scan and now. The
			// sweep's job is that the lease ends up revoked, not that this call
			// is the one that did it.
		default:
			failed = append(failed, fmt.Errorf("revoke expired lease %d: %w", id, err))
		}
	}

	return revoked, errors.Join(failed...)
}
