package config

import (
	"time"

	"github.com/sergeyslonimsky/core/di"
)

const (
	// defaultLeaseMinTTL is the shortest grant honoured. The wire protocol
	// carries TTLs in whole seconds, so anything below one second cannot be
	// expressed back to the client.
	defaultLeaseMinTTL = time.Second

	// defaultLeaseCheckpointThreshold is the drift a renewal may accumulate
	// before it is written, as a fraction of the TTL. Half a TTL means roughly
	// one write per two keepalives at the usual TTL/3 ping rate, and bounds how
	// much earlier than promised a lease can expire after an abrupt restart.
	defaultLeaseCheckpointThreshold = 0.5

	// defaultLeaseSweepInterval is how often expired leases are collected.
	// One second matches the resolution of the TTLs themselves.
	defaultLeaseSweepInterval = time.Second

	// defaultLeaseSweepBatch caps how many leases one sweep revokes, so a large
	// backlog cannot hold the single bbolt writer for an unbounded stretch.
	defaultLeaseSweepBatch = 128
)

// Lease configures lease grants and the expiry sweep.
type Lease struct {
	MinTTL time.Duration
	// MaxTTL of zero means unbounded, as in etcd, and is therefore the default:
	// a long lease costs one record and one index entry, while capping it would
	// break clients holding deliberately long-lived sessions.
	MaxTTL              time.Duration
	CheckpointThreshold float64
	SweepInterval       time.Duration
	SweepBatch          int
}

func newLeaseConfig(cfg *di.Config) Lease {
	threshold := di.Get[float64](cfg, "lease.checkpoint.threshold")
	if threshold <= 0 {
		threshold = defaultLeaseCheckpointThreshold
	}

	return Lease{
		MinTTL: durOrDefault(
			di.Get[time.Duration](cfg, "lease.ttl.min"),
			defaultLeaseMinTTL,
		),
		// Not durOrDefault: zero is a meaningful value here (unbounded), and the
		// helper would replace it with the default.
		MaxTTL:              di.Get[time.Duration](cfg, "lease.ttl.max"),
		CheckpointThreshold: threshold,
		SweepInterval: durOrDefault(
			di.Get[time.Duration](cfg, "lease.sweep.interval"),
			defaultLeaseSweepInterval,
		),
		SweepBatch: intOrDefault(
			di.Get[int](cfg, "lease.sweep.batch"),
			defaultLeaseSweepBatch,
		),
	}
}
