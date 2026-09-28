package lease_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	leaserepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/lease"
	"github.com/sergeyslonimsky/elara/internal/usecase/lease"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

// benchService wires the lease usecase over a real bbolt file — the cost being
// measured is a write transaction, so a mocked repository would measure nothing.
//
// NoSync matches the other benchmarks in this repository, which measure the code
// path rather than the disk. The _Durable pair below keeps the fsync number
// honest; the two modes are not comparable with each other, only across changes.
//
// keyDeleter is nil on purpose: KeepAlive never deletes a key, and a stub would
// only hide it if that ever changed.
func benchService(b *testing.B, threshold float64, noSync bool) *lease.Service {
	b.Helper()

	store, err := bbolt.Open(filepath.Join(b.TempDir(), "elara.db"))
	if err != nil {
		b.Fatalf("open store: %v", err)
	}

	b.Cleanup(func() { _ = store.Close() })

	store.DB().NoSync = noSync

	return lease.New(
		bbolt.NewManager(store.DB()),
		leaserepo.NewRepository(pkgbbolt.NewManager(store.DB())),
		nil,
		lease.Config{CheckpointThreshold: threshold},
	)
}

// benchKeepAlive is the body every variant shares, so a change to the loop
// cannot drift between them.
func benchKeepAlive(b *testing.B, threshold float64, noSync bool) {
	b.Helper()

	svc := benchService(b, threshold, noSync)
	ctx := b.Context()

	l, err := svc.Grant(ctx, 0, time.Hour)
	if err != nil {
		b.Fatalf("grant: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if _, err := svc.KeepAlive(ctx, l.ID); err != nil {
			b.Fatalf("keepalive: %v", err)
		}
	}
}

// BenchmarkKeepAlive_Throttled is the common case: a client pinging at roughly
// TTL/3, where most renewals move the expiry too little to be worth a write.
func BenchmarkKeepAlive_Throttled(b *testing.B) {
	benchKeepAlive(b, 0.5, true)
}

// BenchmarkKeepAlive_AlwaysPersisted is the same call with throttling disabled,
// i.e. what every renewal would cost if the expiry were written each time. The
// gap between the two is the entire argument for the checkpoint threshold — and,
// once the store replicates, the difference in Raft-log volume.
func BenchmarkKeepAlive_AlwaysPersisted(b *testing.B) {
	benchKeepAlive(b, 0, true)
}

// BenchmarkKeepAlive_AlwaysPersisted_Durable is the same again with fsync on
// every commit — the cost an operator actually pays per renewal on a durable
// store, and the reason the threshold is not merely a micro-optimisation.
func BenchmarkKeepAlive_AlwaysPersisted_Durable(b *testing.B) {
	benchKeepAlive(b, 0, false)
}
