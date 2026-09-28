package lease_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/usecase/lease"
)

func TestService_Revoke(t *testing.T) {
	t.Parallel()

	refs := []domain.KeyRef{
		{Namespace: "prod", Path: "/lock/leader"},
		{Namespace: "prod", Path: "/lock/worker"},
	}

	t.Run("deletes every key the lease held", func(t *testing.T) {
		t.Parallel()

		svc, m := setupService(t, lease.Config{})
		expectPassthroughTx(m, 1)
		m.leases.EXPECT().Revoke(gomock.Any(), int64(1)).Return(refs, nil)
		m.keys.EXPECT().DeleteKeys(gomock.Any(), refs, false).Return(nil, int64(9), nil)

		require.NoError(t, svc.Revoke(t.Context(), 1))
	})

	t.Run("a lease holding nothing deletes nothing", func(t *testing.T) {
		t.Parallel()

		svc, m := setupService(t, lease.Config{})
		expectPassthroughTx(m, 1)
		m.leases.EXPECT().Revoke(gomock.Any(), int64(1)).Return(nil, nil)

		// No DeleteKeys expectation: an empty set must not reach the write path,
		// where it would otherwise be a no-op that still opens the door to a
		// wasted revision.
		require.NoError(t, svc.Revoke(t.Context(), 1))
	})

	t.Run("an unknown lease reports not-found", func(t *testing.T) {
		t.Parallel()

		svc, m := setupService(t, lease.Config{})
		expectPassthroughTx(m, 1)
		m.leases.EXPECT().
			Revoke(gomock.Any(), int64(404)).
			Return(nil, storage.ErrResourceNotFound)

		err := svc.Revoke(t.Context(), 404)
		require.ErrorIs(t, err, domain.ErrLeaseNotFound)
	})
}

func TestService_RevokeExpired(t *testing.T) {
	t.Parallel()

	now := time.Now()

	t.Run("revokes each due lease in its own transaction", func(t *testing.T) {
		t.Parallel()

		svc, m := setupService(t, lease.Config{})
		m.leases.EXPECT().ExpiredBefore(gomock.Any(), now, 10).Return([]int64{1, 2}, nil)
		expectPassthroughTx(m, 2)
		m.leases.EXPECT().Revoke(gomock.Any(), int64(1)).Return(nil, nil)
		m.leases.EXPECT().Revoke(gomock.Any(), int64(2)).Return(nil, nil)

		revoked, err := svc.RevokeExpired(t.Context(), now, 10)
		require.NoError(t, err)
		assert.Equal(t, 2, revoked)
	})

	t.Run("a lease that vanished mid-sweep is not an error", func(t *testing.T) {
		t.Parallel()

		// A client revoked it between the scan and the write. The sweep's job is
		// that the lease ends up revoked, not that this call did it.
		svc, m := setupService(t, lease.Config{})
		m.leases.EXPECT().ExpiredBefore(gomock.Any(), now, 0).Return([]int64{1}, nil)
		expectPassthroughTx(m, 1)
		m.leases.EXPECT().
			Revoke(gomock.Any(), int64(1)).
			Return(nil, storage.ErrResourceNotFound)

		revoked, err := svc.RevokeExpired(t.Context(), now, 0)
		require.NoError(t, err)
		assert.Zero(t, revoked)
	})

	t.Run("one stuck lease does not hold up the rest", func(t *testing.T) {
		t.Parallel()

		// A locked namespace is the realistic case: an operator froze it, and the
		// keys of a lease living there cannot be deleted right now.
		locked := errors.New("namespace is locked")

		svc, m := setupService(t, lease.Config{})
		m.leases.EXPECT().ExpiredBefore(gomock.Any(), now, 0).Return([]int64{1, 2}, nil)
		expectPassthroughTx(m, 2)
		m.leases.EXPECT().
			Revoke(gomock.Any(), int64(1)).
			Return([]domain.KeyRef{{Namespace: "frozen", Path: "/a"}}, nil)
		m.keys.EXPECT().DeleteKeys(gomock.Any(), gomock.Any(), false).Return(nil, int64(0), locked)
		m.leases.EXPECT().Revoke(gomock.Any(), int64(2)).Return(nil, nil)

		revoked, err := svc.RevokeExpired(t.Context(), now, 0)
		require.ErrorIs(t, err, locked)
		assert.Equal(t, 1, revoked, "the healthy lease is still revoked")
	})

	t.Run("nothing due is not an error", func(t *testing.T) {
		t.Parallel()

		svc, m := setupService(t, lease.Config{})
		m.leases.EXPECT().ExpiredBefore(gomock.Any(), now, 0).Return(nil, nil)

		revoked, err := svc.RevokeExpired(t.Context(), now, 0)
		require.NoError(t, err)
		assert.Zero(t, revoked)
	})
}
