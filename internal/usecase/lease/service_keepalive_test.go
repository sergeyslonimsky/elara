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

func TestService_KeepAlive(t *testing.T) {
	t.Parallel()

	const ttl = 30 * time.Second

	cfg := lease.Config{CheckpointThreshold: 0.5}

	t.Run("a small renewal answers without touching the store", func(t *testing.T) {
		t.Parallel()

		// 5s of drift on a 30s lease is under the half-TTL threshold. This is the
		// common case — a client pings every ~TTL/3 — and it is exactly the write
		// the throttle exists to avoid.
		svc, m := setupService(t, cfg)
		m.leases.EXPECT().Get(gomock.Any(), int64(1)).Return(liveLease(1, ttl, 25*time.Second), nil)

		l, err := svc.KeepAlive(t.Context(), 1)
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now().Add(ttl), l.ExpiresAt, time.Second,
			"the client is promised a full TTL even when nothing was written")
	})

	t.Run("a large renewal is persisted", func(t *testing.T) {
		t.Parallel()

		svc, m := setupService(t, cfg)
		m.leases.EXPECT().Get(gomock.Any(), int64(1)).Return(liveLease(1, ttl, time.Second), nil)
		expectPassthroughTx(m, 1)
		m.leases.EXPECT().Renew(gomock.Any(), int64(1), gomock.Any()).Return(nil)

		l, err := svc.KeepAlive(t.Context(), 1)
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now().Add(ttl), l.ExpiresAt, time.Second)
	})

	t.Run("an expired lease is not resurrected", func(t *testing.T) {
		t.Parallel()

		// Its keys may already be gone, so the lease the client thinks it holds
		// no longer exists in any useful sense.
		svc, m := setupService(t, cfg)
		m.leases.EXPECT().
			Get(gomock.Any(), int64(1)).
			Return(liveLease(1, ttl, -time.Second), nil)

		_, err := svc.KeepAlive(t.Context(), 1)
		require.ErrorIs(t, err, domain.ErrLeaseExpired)
	})

	t.Run("an unknown lease reports not-found", func(t *testing.T) {
		t.Parallel()

		svc, m := setupService(t, cfg)
		m.leases.EXPECT().Get(gomock.Any(), int64(404)).Return(nil, storage.ErrResourceNotFound)

		_, err := svc.KeepAlive(t.Context(), 404)
		require.ErrorIs(t, err, domain.ErrLeaseNotFound)
	})

	t.Run("a zero threshold persists every renewal", func(t *testing.T) {
		t.Parallel()

		svc, m := setupService(t, lease.Config{CheckpointThreshold: 0})
		m.leases.EXPECT().Get(gomock.Any(), int64(1)).Return(liveLease(1, ttl, 29*time.Second), nil)
		expectPassthroughTx(m, 1)
		m.leases.EXPECT().Renew(gomock.Any(), int64(1), gomock.Any()).Return(nil)

		_, err := svc.KeepAlive(t.Context(), 1)
		require.NoError(t, err)
	})
}
