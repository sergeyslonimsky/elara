package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

func liveLease(id int64) *domain.Lease {
	return &domain.Lease{
		ID:        id,
		TTL:       time.Minute,
		GrantedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Minute),
	}
}

func TestService_PutKey_LeaseClaims(t *testing.T) {
	t.Parallel()

	const (
		namespace = "prod"
		path      = "/lock/leader"
	)

	ref := domain.KeyRef{Namespace: namespace, Path: path}

	tests := []struct {
		name     string
		lease    domain.LeaseAssignment
		mockFunc func(m mocks)
	}{
		{
			name:  "attaching a new key records the claim",
			lease: domain.LeaseAssignment{ID: 5},
			mockFunc: func(m mocks) {
				m.leases.EXPECT().Get(gomock.Any(), int64(5)).Return(liveLease(5), nil)
				m.kv.EXPECT().
					PutKey(gomock.Any(), namespace, path, []byte("v"), domain.LeaseAssignment{ID: 5}).
					Return(nil, int64(1), nil)
				m.leases.EXPECT().AttachKey(gomock.Any(), int64(5), ref).Return(nil)
			},
		},
		{
			name:  "moving to another lease detaches the old claim first",
			lease: domain.LeaseAssignment{ID: 6},
			mockFunc: func(m mocks) {
				m.leases.EXPECT().Get(gomock.Any(), int64(6)).Return(liveLease(6), nil)
				m.kv.EXPECT().
					PutKey(gomock.Any(), namespace, path, []byte("v"), domain.LeaseAssignment{ID: 6}).
					Return(&domain.KVPair{Namespace: namespace, Path: path, Lease: 5}, int64(2), nil)
				m.leases.EXPECT().DetachKey(gomock.Any(), int64(5), ref).Return(nil)
				m.leases.EXPECT().AttachKey(gomock.Any(), int64(6), ref).Return(nil)
			},
		},
		{
			name:  "a zero lease detaches without reattaching",
			lease: domain.LeaseAssignment{},
			mockFunc: func(m mocks) {
				m.kv.EXPECT().
					PutKey(gomock.Any(), namespace, path, []byte("v"), domain.LeaseAssignment{}).
					Return(&domain.KVPair{Namespace: namespace, Path: path, Lease: 5}, int64(2), nil)
				m.leases.EXPECT().DetachKey(gomock.Any(), int64(5), ref).Return(nil)
			},
		},
		{
			name:  "rewriting a key under the same lease touches no index",
			lease: domain.LeaseAssignment{ID: 5},
			mockFunc: func(m mocks) {
				m.leases.EXPECT().Get(gomock.Any(), int64(5)).Return(liveLease(5), nil)
				m.kv.EXPECT().
					PutKey(gomock.Any(), namespace, path, []byte("v"), domain.LeaseAssignment{ID: 5}).
					Return(&domain.KVPair{Namespace: namespace, Path: path, Lease: 5}, int64(2), nil)
			},
		},
		{
			name:  "ignore_lease keeps the existing claim in place",
			lease: domain.LeaseAssignment{Ignore: true},
			mockFunc: func(m mocks) {
				// The existence check that ignore_lease needs, satisfied.
				m.kv.EXPECT().
					RangeQuery(gomock.Any(), namespace, path, "", "", int64(1), int64(0), true).
					Return([]*domain.KVPair{{Namespace: namespace, Path: path, Lease: 5}}, false, nil)
				m.kv.EXPECT().
					PutKey(gomock.Any(), namespace, path, []byte("v"), domain.LeaseAssignment{Ignore: true}).
					Return(&domain.KVPair{Namespace: namespace, Path: path, Lease: 5}, int64(2), nil)
			},
		},
		{
			name:  "a write with no lease at all never consults the index",
			lease: domain.LeaseAssignment{},
			mockFunc: func(m mocks) {
				m.kv.EXPECT().
					PutKey(gomock.Any(), namespace, path, []byte("v"), domain.LeaseAssignment{}).
					Return(nil, int64(1), nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			captureNotifications(m, &notifyCapture{})

			m.schemaValidator.EXPECT().
				Validate(gomock.Any(), namespace, path, "v", gomock.Any()).
				Return(nil)
			expectPassthroughTx(m, 1)
			tt.mockFunc(m)

			_, _, err := svc.PutKey(t.Context(), namespace, path, []byte("v"), tt.lease)
			require.NoError(t, err)
		})
	}
}

func TestService_PutKey_LeaseRejections(t *testing.T) {
	t.Parallel()

	const (
		namespace = "prod"
		path      = "/lock/leader"
	)

	tests := []struct {
		name     string
		lease    domain.LeaseAssignment
		mockFunc func(m mocks)
		errIs    error
	}{
		{
			name:  "unknown lease",
			lease: domain.LeaseAssignment{ID: 404},
			mockFunc: func(m mocks) {
				m.leases.EXPECT().
					Get(gomock.Any(), int64(404)).
					Return(nil, storage.ErrResourceNotFound)
			},
			errIs: domain.ErrLeaseNotFound,
		},
		{
			name:  "expired lease",
			lease: domain.LeaseAssignment{ID: 7},
			mockFunc: func(m mocks) {
				m.leases.EXPECT().Get(gomock.Any(), int64(7)).Return(&domain.Lease{
					ID:        7,
					TTL:       time.Minute,
					ExpiresAt: time.Now().Add(-time.Minute),
				}, nil)
			},
			errIs: domain.ErrLeaseExpired,
		},
		{
			name:  "ignore_lease on a key that does not exist",
			lease: domain.LeaseAssignment{Ignore: true},
			mockFunc: func(m mocks) {
				m.kv.EXPECT().
					RangeQuery(gomock.Any(), namespace, path, "", "", int64(1), int64(0), true).
					Return(nil, false, nil)
			},
			errIs: domain.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			m.schemaValidator.EXPECT().
				Validate(gomock.Any(), namespace, path, "v", gomock.Any()).
				Return(nil)
			expectPassthroughTx(m, 1)
			tt.mockFunc(m)

			// No PutKey expectation on purpose: the guard runs before the write,
			// so a rejected Put must not reach storage at all.
			_, _, err := svc.PutKey(t.Context(), namespace, path, []byte("v"), tt.lease)
			require.ErrorIs(t, err, tt.errIs)
		})
	}
}

func TestService_DeleteKeys(t *testing.T) {
	t.Parallel()

	refs := []domain.KeyRef{
		{Namespace: "prod", Path: "/lock/leader"},
		{Namespace: "prod", Path: "/plain.json"},
	}

	svc, m, _ := setupService(t)
	capture := &notifyCapture{}
	captureNotifications(m, capture)

	expectPassthroughTx(m, 1)
	m.kv.EXPECT().
		DeleteKeys(gomock.Any(), refs, false).
		Return([]*domain.KVPair{
			{Namespace: "prod", Path: "/lock/leader", Lease: 5},
			{Namespace: "prod", Path: "/plain.json"},
		}, int64(9), nil)
	// Only the leased key releases a claim; the unleased one has none to release.
	m.leases.EXPECT().
		DetachKey(gomock.Any(), int64(5), domain.KeyRef{Namespace: "prod", Path: "/lock/leader"}).
		Return(nil)

	deleted, rev, err := svc.DeleteKeys(t.Context(), refs, false)
	require.NoError(t, err)
	require.Len(t, deleted, 2)
	assert.Equal(t, int64(9), rev)

	_, _, published := capture.snapshot()
	require.Len(t, published, 2, "every deleted key is published, leased or not")
	assert.Equal(t, int64(9), published[0].revision)
	assert.Equal(t, int64(9), published[1].revision, "one revision for the whole set")
}

func TestService_DeleteRangeKeys_ReleasesLeaseClaims(t *testing.T) {
	t.Parallel()

	// A key deleted through the ordinary range path has to release its claim too,
	// otherwise the index keeps growing and points at keys that no longer exist.
	svc, m, _ := setupService(t)
	captureNotifications(m, &notifyCapture{})

	expectPassthroughTx(m, 1)
	m.kv.EXPECT().
		DeleteRangeKeys(gomock.Any(), "prod", "/lock/leader", "", "", false).
		Return([]*domain.KVPair{{Namespace: "prod", Path: "/lock/leader", Lease: 5}}, int64(3), nil)
	m.leases.EXPECT().
		DetachKey(gomock.Any(), int64(5), domain.KeyRef{Namespace: "prod", Path: "/lock/leader"}).
		Return(nil)

	_, _, err := svc.DeleteRangeKeys(t.Context(), "prod", "/lock/leader", "", "", false)
	require.NoError(t, err)
}
