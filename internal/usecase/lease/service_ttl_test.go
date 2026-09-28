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

func TestService_TimeToLive(t *testing.T) {
	t.Parallel()

	const ttl = time.Minute

	t.Run("without keys the index is not read", func(t *testing.T) {
		t.Parallel()

		svc, m := setupService(t, lease.Config{})
		m.leases.EXPECT().Get(gomock.Any(), int64(1)).Return(liveLease(1, ttl, 30*time.Second), nil)

		l, keys, err := svc.TimeToLive(t.Context(), 1, false)
		require.NoError(t, err)
		assert.Nil(t, keys)
		// Truncated down to whole seconds, so the microseconds spent getting here
		// are enough to make this 29s rather than 30s — asserting equality would
		// be asserting on scheduling.
		assert.InDelta(
			t,
			float64(30*time.Second),
			float64(l.RemainingTTL(time.Now())),
			float64(time.Second),
		)
	})

	t.Run("with keys the attached set comes back", func(t *testing.T) {
		t.Parallel()

		refs := []domain.KeyRef{{Namespace: "prod", Path: "/lock/leader"}}

		svc, m := setupService(t, lease.Config{})
		m.leases.EXPECT().Get(gomock.Any(), int64(1)).Return(liveLease(1, ttl, time.Second), nil)
		m.leases.EXPECT().KeysOf(gomock.Any(), int64(1)).Return(refs, nil)

		_, keys, err := svc.TimeToLive(t.Context(), 1, true)
		require.NoError(t, err)
		assert.Equal(t, refs, keys)
	})

	t.Run("an expired lease the sweep has not reached is still reported", func(t *testing.T) {
		t.Parallel()

		// Otherwise the answer would depend on sweep timing rather than on what
		// the store holds.
		svc, m := setupService(t, lease.Config{})
		m.leases.EXPECT().Get(gomock.Any(), int64(1)).Return(liveLease(1, ttl, -time.Second), nil)

		l, _, err := svc.TimeToLive(t.Context(), 1, false)
		require.NoError(t, err)
		assert.Zero(t, l.RemainingTTL(time.Now()), "expired means nothing left, not negative")
	})

	t.Run("an unknown lease reports not-found", func(t *testing.T) {
		t.Parallel()

		svc, m := setupService(t, lease.Config{})
		m.leases.EXPECT().Get(gomock.Any(), int64(404)).Return(nil, storage.ErrResourceNotFound)

		_, _, err := svc.TimeToLive(t.Context(), 404, false)
		require.ErrorIs(t, err, domain.ErrLeaseNotFound)
	})
}

func TestService_List(t *testing.T) {
	t.Parallel()

	svc, m := setupService(t, lease.Config{})
	want := []*domain.Lease{liveLease(1, time.Minute, time.Second), liveLease(2, time.Hour, time.Hour)}
	m.leases.EXPECT().List(gomock.Any()).Return(want, nil)

	got, err := svc.List(t.Context())
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
