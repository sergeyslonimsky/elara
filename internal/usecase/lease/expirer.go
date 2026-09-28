package lease

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const (
	// fallbackInterval and fallbackBatch apply when a caller passes nothing
	// usable. Zero is not a meaningful pace — time.NewTicker panics on it — and
	// a zero batch would mean "no cap", which is the opposite of what an unset
	// value should buy. Config supplies real values in production; this keeps a
	// hand-assembled Service (tests, tools) from booting a broken sweep.
	fallbackInterval = time.Second
	fallbackBatch    = 128
)

// Expirer is the background sweep that revokes leases whose time has come.
//
// It only decides *when* to look; what has expired is decided inside the
// transaction, from stored state and the instant it passes in (see
// Service.RevokeExpired). That split is what keeps the sweep correct across a
// restart — nothing is remembered between ticks — and what will keep it correct
// once the store replicates, where a tick on one node must not be the thing that
// makes a lease expire.
//
// A missed tick is therefore harmless: the next one picks up everything that
// came due in between, bounded by the batch size.
type Expirer struct {
	svc      *Service
	interval time.Duration
	batch    int
}

func NewExpirer(svc *Service, interval time.Duration, batch int) *Expirer {
	if interval <= 0 {
		interval = fallbackInterval
	}

	if batch <= 0 {
		batch = fallbackBatch
	}

	return &Expirer{svc: svc, interval: interval, batch: batch}
}

// Run sweeps until ctx is cancelled. It is started as a goroutine at boot and
// stops when the process starts shutting down.
func (e *Expirer) Run(ctx context.Context) error {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("lease expirer stopped: %w", ctx.Err())
		case <-ticker.C:
			e.sweep(ctx)
		}
	}
}

// sweep runs one pass and never propagates an error.
//
// A lease that cannot be revoked right now — its namespace is locked, say — must
// not stop the loop: the next tick retries it, and revoking is idempotent. The
// error is logged rather than swallowed so that a permanently stuck lease is
// visible instead of silently outliving its TTL.
func (e *Expirer) sweep(ctx context.Context) {
	revoked, err := e.svc.RevokeExpired(ctx, time.Now(), e.batch)
	if err != nil {
		slog.ErrorContext(ctx, "lease sweep: some leases could not be revoked",
			slog.Any("error", err),
			slog.Int("revoked", revoked),
		)

		return
	}

	if revoked > 0 {
		slog.DebugContext(ctx, "lease sweep: revoked expired leases", slog.Int("count", revoked))
	}
}
