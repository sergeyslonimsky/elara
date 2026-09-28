package internal

import (
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// LeaseMeta is the on-disk JSON shape for a domain.Lease. The primary bucket
// key is the big-endian lease ID; two secondary indexes hold composite keys —
// `lease_keys` for "which keys does this lease hold" and `lease_expiry` for
// "which leases are due", ordered by expiry so the sweep is a prefix scan
// rather than a full walk.
//
// TTL is stored in nanoseconds rather than as a time.Duration string so the
// record stays a plain number on disk; the wire protocol carries seconds and
// the entity carries a Duration, and neither is the on-disk shape.
type LeaseMeta struct {
	ID        int64     `json:"id"`
	TTLNanos  int64     `json:"ttl_nanos"`
	GrantedAt time.Time `json:"granted_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func DomainToLeaseMeta(l *domain.Lease) LeaseMeta {
	return LeaseMeta{
		ID:        l.ID,
		TTLNanos:  l.TTL.Nanoseconds(),
		GrantedAt: l.GrantedAt,
		ExpiresAt: l.ExpiresAt,
	}
}

func LeaseMetaToDomain(m LeaseMeta) *domain.Lease {
	return &domain.Lease{
		ID:        m.ID,
		TTL:       time.Duration(m.TTLNanos),
		GrantedAt: m.GrantedAt,
		ExpiresAt: m.ExpiresAt,
	}
}
