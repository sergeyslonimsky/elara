package lease_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/usecase/lease"
)

func TestService_Grant(t *testing.T) {
	t.Parallel()

	t.Run("honours a caller-supplied id", func(t *testing.T) {
		t.Parallel()

		svc, m := setupService(t, lease.Config{})
		expectPassthroughTx(m, 1)
		m.leases.EXPECT().
			Grant(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ any, l *domain.Lease) error {
				assert.Equal(t, int64(42), l.ID)

				return nil
			})

		l, err := svc.Grant(t.Context(), 42, 30*time.Second)
		require.NoError(t, err)
		assert.Equal(t, int64(42), l.ID)
		assert.Equal(t, 30*time.Second, l.TTL)
		assert.Equal(t, l.GrantedAt.Add(30*time.Second), l.ExpiresAt)
	})

	t.Run("generates an id when asked for zero", func(t *testing.T) {
		t.Parallel()

		svc, m := setupService(t, lease.Config{})
		expectPassthroughTx(m, 1)
		m.leases.EXPECT().Grant(gomock.Any(), gomock.Any()).Return(nil)

		l, err := svc.Grant(t.Context(), 0, time.Minute)
		require.NoError(t, err)
		assert.Positive(t, l.ID)
	})

	t.Run("a taken caller-supplied id is an error, not a reuse", func(t *testing.T) {
		t.Parallel()

		// Two clients sharing one lease would expire each other's keys.
		svc, m := setupService(t, lease.Config{})
		expectPassthroughTx(m, 1)
		m.leases.EXPECT().
			Grant(gomock.Any(), gomock.Any()).
			Return(storage.ErrResourceAlreadyExists)

		_, err := svc.Grant(t.Context(), 42, time.Minute)
		require.ErrorIs(t, err, domain.ErrLeaseExists)
	})

	t.Run("a generated id that collides is redrawn", func(t *testing.T) {
		t.Parallel()

		svc, m := setupService(t, lease.Config{})
		expectPassthroughTx(m, 2)
		gomock.InOrder(
			m.leases.EXPECT().
				Grant(gomock.Any(), gomock.Any()).
				Return(storage.ErrResourceAlreadyExists),
			m.leases.EXPECT().Grant(gomock.Any(), gomock.Any()).Return(nil),
		)

		l, err := svc.Grant(t.Context(), 0, time.Minute)
		require.NoError(t, err)
		assert.Positive(t, l.ID)
	})

	t.Run("collisions all the way point at broken randomness", func(t *testing.T) {
		t.Parallel()

		svc, m := setupService(t, lease.Config{})
		expectPassthroughTx(m, 3)
		m.leases.EXPECT().
			Grant(gomock.Any(), gomock.Any()).
			Times(3).
			Return(storage.ErrResourceAlreadyExists)

		_, err := svc.Grant(t.Context(), 0, time.Minute)
		require.ErrorIs(t, err, domain.ErrLeaseIDExhausted)
	})
}

func TestService_Grant_TTLBounds(t *testing.T) {
	t.Parallel()

	cfg := lease.Config{MinTTL: 5 * time.Second, MaxTTL: time.Hour}

	tests := []struct {
		name      string
		requested time.Duration
		want      time.Duration
		errIs     error
	}{
		{
			name:      "a ttl inside the bounds is granted as asked",
			requested: time.Minute,
			want:      time.Minute,
		},
		{
			// Clamped rather than rejected: the response tells the client the
			// granted TTL, and it paces its keepalives by that.
			name:      "a ttl below the minimum is raised",
			requested: time.Second,
			want:      5 * time.Second,
		},
		{
			name:      "a ttl above the maximum is lowered",
			requested: 24 * time.Hour,
			want:      time.Hour,
		},
		{
			name:      "a zero ttl is rejected outright",
			requested: 0,
			errIs:     domain.ErrLeaseTTLInvalid,
		},
		{
			name:      "a negative ttl is rejected outright",
			requested: -time.Second,
			errIs:     domain.ErrLeaseTTLInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m := setupService(t, cfg)

			if tt.errIs == nil {
				expectPassthroughTx(m, 1)
				m.leases.EXPECT().Grant(gomock.Any(), gomock.Any()).Return(nil)
			}

			l, err := svc.Grant(t.Context(), 1, tt.requested)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, l.TTL)
		})
	}
}
