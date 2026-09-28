package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestNewLease(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 28, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		ttl   time.Duration
		errIs error
	}{
		{
			name:  "positive ttl grants a lease",
			ttl:   30 * time.Second,
			errIs: nil,
		},
		{
			name:  "zero ttl is rejected",
			ttl:   0,
			errIs: domain.ErrLeaseTTLInvalid,
		},
		{
			name:  "negative ttl is rejected",
			ttl:   -time.Second,
			errIs: domain.ErrLeaseTTLInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			l, err := domain.NewLease(42, tt.ttl, now)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)
				assert.Nil(t, l)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, int64(42), l.ID)
			assert.Equal(t, tt.ttl, l.TTL)
			assert.Equal(t, now, l.GrantedAt)
			assert.Equal(t, now.Add(tt.ttl), l.ExpiresAt)
		})
	}
}

func TestLease_IsExpired(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 28, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "expiry in the future is not expired",
			expiresAt: now.Add(time.Second),
			want:      false,
		},
		{
			name:      "expiry in the past is expired",
			expiresAt: now.Add(-time.Second),
			want:      true,
		},
		{
			// Exclusive boundary, same as Session.IsExpired: the expiry instant
			// itself still counts as alive.
			name:      "expiry exactly at now is not expired",
			expiresAt: now,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			l := domain.Lease{ExpiresAt: tt.expiresAt}
			assert.Equal(t, tt.want, l.IsExpired(now))
		})
	}
}

func TestLease_EnsureLive(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 28, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		expiresAt time.Time
		errIs     error
	}{
		{
			name:      "live lease returns nil",
			expiresAt: now.Add(time.Minute),
			errIs:     nil,
		},
		{
			name:      "expired lease returns ErrLeaseExpired",
			expiresAt: now.Add(-time.Minute),
			errIs:     domain.ErrLeaseExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			l := domain.Lease{ExpiresAt: tt.expiresAt}
			err := l.EnsureLive(now)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestLease_RenewedExpiry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 28, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		ttl       time.Duration
		expiresAt time.Time
		renewAt   time.Time
		want      time.Time
	}{
		{
			name:      "renewal is a full ttl from the renewal instant",
			ttl:       30 * time.Second,
			expiresAt: now.Add(10 * time.Second),
			renewAt:   now,
			want:      now.Add(30 * time.Second),
		},
		{
			// etcd resets the clock instead of extending the old expiry, so a
			// lease renewed late does not bank the time it sat idle.
			name:      "late renewal does not accumulate idle time",
			ttl:       30 * time.Second,
			expiresAt: now.Add(-5 * time.Second),
			renewAt:   now,
			want:      now.Add(30 * time.Second),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			l := domain.Lease{TTL: tt.ttl, ExpiresAt: tt.expiresAt}
			assert.Equal(t, tt.want, l.RenewedExpiry(tt.renewAt))
		})
	}
}

func TestLease_RemainingTTL(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 28, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		expiresAt time.Time
		want      time.Duration
	}{
		{
			name:      "whole seconds pass through",
			expiresAt: now.Add(10 * time.Second),
			want:      10 * time.Second,
		},
		{
			// The wire protocol carries TTLs in seconds, so a partial second is
			// truncated rather than rounded up.
			name:      "sub-second remainder is truncated down",
			expiresAt: now.Add(4700 * time.Millisecond),
			want:      4 * time.Second,
		},
		{
			name:      "less than a second left reports zero",
			expiresAt: now.Add(900 * time.Millisecond),
			want:      0,
		},
		{
			name:      "expired lease reports zero, never negative",
			expiresAt: now.Add(-time.Hour),
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			l := domain.Lease{ExpiresAt: tt.expiresAt}
			assert.Equal(t, tt.want, l.RemainingTTL(now))
		})
	}
}

func TestLease_NeedsCheckpoint(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 28, 12, 0, 0, 0, time.UTC)

	const ttl = 30 * time.Second

	tests := []struct {
		name      string
		expiresAt time.Time
		newExpiry time.Time
		threshold float64
		want      bool
	}{
		{
			name:      "drift below the threshold is not worth a write",
			expiresAt: now.Add(20 * time.Second),
			newExpiry: now.Add(ttl),
			threshold: 0.5,
			want:      false,
		},
		{
			name:      "drift exactly at the threshold writes",
			expiresAt: now.Add(15 * time.Second),
			newExpiry: now.Add(ttl),
			threshold: 0.5,
			want:      true,
		},
		{
			name:      "drift beyond the threshold writes",
			expiresAt: now.Add(time.Second),
			newExpiry: now.Add(ttl),
			threshold: 0.5,
			want:      true,
		},
		{
			name:      "no drift at all writes nothing",
			expiresAt: now.Add(ttl),
			newExpiry: now.Add(ttl),
			threshold: 0.5,
			want:      false,
		},
		{
			// A renewal that arrives out of order carries an older expiry than
			// the store already holds; writing it would move the expiry backwards.
			name:      "renewal older than the stored expiry writes nothing",
			expiresAt: now.Add(ttl),
			newExpiry: now.Add(10 * time.Second),
			threshold: 0.5,
			want:      false,
		},
		{
			name:      "zero threshold persists every renewal",
			expiresAt: now.Add(29 * time.Second),
			newExpiry: now.Add(ttl),
			threshold: 0,
			want:      true,
		},
		{
			name:      "negative threshold persists every renewal",
			expiresAt: now.Add(29 * time.Second),
			newExpiry: now.Add(ttl),
			threshold: -1,
			want:      true,
		},
		{
			// A full TTL of drift means the stored expiry has just reached the
			// present, which is the last instant the expiry sweep still sees the
			// lease as live.
			name:      "threshold of one writes once the stored expiry reaches now",
			expiresAt: now,
			newExpiry: now.Add(ttl),
			threshold: 1,
			want:      true,
		},
		{
			// Misconfiguration must not be able to raise the ceiling: without the
			// clamp a threshold of 5 would tolerate 150s of drift on a 30s lease,
			// and the sweep would revoke it mid-renewal.
			name:      "threshold above one is clamped to one TTL of drift",
			expiresAt: now,
			newExpiry: now.Add(ttl),
			threshold: 5,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			l := domain.Lease{TTL: ttl, ExpiresAt: tt.expiresAt}
			assert.Equal(t, tt.want, l.NeedsCheckpoint(tt.newExpiry, tt.threshold))
		})
	}
}

func TestNewLeaseID(t *testing.T) {
	t.Parallel()

	t.Run("is positive and non-zero", func(t *testing.T) {
		t.Parallel()

		for range 100 {
			id, err := domain.NewLeaseID()
			require.NoError(t, err)
			// Zero spells "no lease" on the wire and a negative id means nothing
			// to an etcd client.
			assert.Positive(t, id)
		}
	})

	t.Run("uniqueness across 1000 calls", func(t *testing.T) {
		t.Parallel()

		const calls = 1000

		seen := make(map[int64]struct{}, calls)

		for range calls {
			id, err := domain.NewLeaseID()
			require.NoError(t, err)

			seen[id] = struct{}{}
		}

		assert.Len(t, seen, calls)
	})
}
