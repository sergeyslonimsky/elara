package domain

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"
)

// Lease is a time-to-live grant that keys attach to: when it expires, every key
// attached to it is deleted. It backs the etcd-compatible gRPC API's Lease RPCs,
// which is how etcd clients build distributed locks and leader election (see
// clientv3/concurrency).
//
// ExpiresAt is the *persisted* expiry, not necessarily the one the client was
// last promised: KeepAlive answers immediately and writes to the store only once
// a renewal is large enough to be worth the write (see NeedsCheckpoint). The gap
// is bounded by the checkpoint threshold, and is the same trade etcd makes when
// lease checkpointing is off.
//
// Expiry lives in the record rather than in a process-local timer on purpose.
// The store is what replicates: once Elara gains Raft-based HA (README roadmap),
// a node holding the expiry in memory stops being the place where the truth is.
type Lease struct {
	ID        int64
	TTL       time.Duration
	GrantedAt time.Time
	ExpiresAt time.Time
}

// leaseIDBytes is the size in bytes of the random lease identifier.
const leaseIDBytes = 8

// NewLease grants a lease running for ttl from now.
//
// Only the entity-level invariant is enforced here — a lease with a
// non-positive TTL is meaningless. Deployment-level bounds (minimum and maximum
// acceptable TTL) are configuration, so the usecase checks those.
func NewLease(id int64, ttl time.Duration, now time.Time) (*Lease, error) {
	if ttl <= 0 {
		return nil, fmt.Errorf("lease ttl %s: %w", ttl, ErrLeaseTTLInvalid)
	}

	return &Lease{
		ID:        id,
		TTL:       ttl,
		GrantedAt: now,
		ExpiresAt: now.Add(ttl),
	}, nil
}

// NewLeaseID generates a random positive lease identifier.
//
// Random rather than drawn from a counter, and generated before the record is
// written: a shared counter would be a coordination point between nodes once the
// store replicates, whereas a value drawn up front travels inside the record and
// keeps apply deterministic. This is the reasoning etcd's lessor follows.
//
// The sign bit is shifted out — the wire type is int64 and a negative lease id
// means nothing to an etcd client. Zero is redrawn because zero is how the wire
// protocol spells "no lease".
func NewLeaseID() (int64, error) {
	b := make([]byte, leaseIDBytes)

	for {
		if _, err := rand.Read(b); err != nil {
			return 0, fmt.Errorf("generate lease id: %w", err)
		}

		if id := int64(binary.BigEndian.Uint64(b) >> 1); id != 0 {
			return id, nil
		}
	}
}

// LeaseAssignment is the lease half of a Put: which lease the key should end up
// attached to, mirroring the wire protocol's two fields.
//
// The zero value means "no lease", which is what an ordinary write wants — so a
// caller with no interest in leases can leave it out entirely. That is the reason
// this is a struct and not a *int64 whose nil had to mean ignore_lease: with the
// pointer, every forgotten field silently became "keep the existing attachment",
// and a Put on a key that did not exist started failing.
//
// The two fields cannot be collapsed: on the wire, lease 0 with ignore_lease set
// and lease 0 without it mean different things.
type LeaseAssignment struct {
	// ID is the lease to attach the key to. Zero detaches it.
	ID int64
	// Ignore is the wire protocol's ignore_lease: keep whatever lease the key
	// already has, which requires the key to exist.
	Ignore bool
}

// Resolve returns the lease the key ends up attached to, given its state before
// the Put (nil for a key that did not exist).
//
// It lives in the entity because two layers have to agree on the answer —
// storage writes the resolved value into the record, and the usecase moves the
// lease-to-keys index by it. A second copy of these lines in either place is a
// divergence waiting to happen.
func (a LeaseAssignment) Resolve(prev *KVPair) int64 {
	if !a.Ignore {
		return a.ID
	}

	if prev == nil {
		return 0
	}

	return prev.Lease
}

// IsExpired reports whether the lease is past its expiry. The boundary is
// exclusive — a lease is still alive at exactly ExpiresAt — matching
// Session.IsExpired.
func (l *Lease) IsExpired(now time.Time) bool {
	return now.After(l.ExpiresAt)
}

// EnsureLive is the guard write paths call before attaching a key: attaching to
// an expired lease would create a key that the next expiry sweep deletes.
func (l *Lease) EnsureLive(now time.Time) error {
	if l.IsExpired(now) {
		return ErrLeaseExpired
	}

	return nil
}

// RenewedExpiry is the expiry a KeepAlive at now would grant: a full TTL from
// now. etcd resets the clock on renewal rather than extending the old expiry, so
// a lease renewed late does not accumulate the time it was idle.
func (l *Lease) RenewedExpiry(now time.Time) time.Time {
	return now.Add(l.TTL)
}

// RemainingTTL is how long the lease has left, truncated to whole seconds
// because the wire protocol carries TTLs in seconds.
//
// Never negative: an expired lease has nothing left. The "no such lease" answer
// is -1 on the wire, which is the handler's encoding decision, not a property of
// an entity that exists.
func (l *Lease) RemainingTTL(now time.Time) time.Duration {
	if l.IsExpired(now) {
		return 0
	}

	return l.ExpiresAt.Sub(now).Truncate(time.Second)
}

// NeedsCheckpoint reports whether renewing to newExpiry earns a write to the
// store.
//
// KeepAlive arrives roughly every TTL/3 per session, so persisting every renewal
// would put a write transaction on every ping — and, once the store replicates,
// an entry in the Raft log. Writing only when a renewal moves ExpiresAt by more
// than some fraction of the TTL keeps writes proportional to lease lifetime
// instead of to client chatter. threshold is that fraction: 0.5 means "half a
// TTL of drift is tolerable".
//
// The cost is bounded staleness — after an abrupt restart a lease expires up to
// threshold×TTL earlier than the client was last promised.
//
// The effective threshold is clamped to (0, 1]. One whole TTL of drift is the
// hard ceiling: newExpiry is a full TTL ahead of the renewal instant, so a drift
// of TTL means the *stored* expiry has just reached the present. Tolerating more
// would let the expiry sweep — which reads the store, not this promise — revoke a
// lease its client is actively renewing. A non-positive threshold degenerates to
// "persist every renewal".
func (l *Lease) NeedsCheckpoint(newExpiry time.Time, threshold float64) bool {
	drift := newExpiry.Sub(l.ExpiresAt)
	if drift <= 0 {
		return false
	}

	if threshold <= 0 {
		return true
	}

	if threshold > 1 {
		threshold = 1
	}

	return drift >= time.Duration(threshold*float64(l.TTL))
}
