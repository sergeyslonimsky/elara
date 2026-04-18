package etcdv3

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
)

// stubRepo is a minimal MaintenanceRepo for stub tests.
type stubRepo struct {
	rev int64
	err error
}

func (s *stubRepo) CurrentRevisionValue(_ context.Context) (int64, error) {
	return s.rev, s.err
}

func TestMaintenance_Status(t *testing.T) {
	t.Parallel()

	r := &stubRepo{rev: 42}
	m := NewMaintenanceServer(r)

	resp, err := m.Status(context.Background(), &etcdserverpb.StatusRequest{})
	require.NoError(t, err)

	assert.Equal(t, etcdVersion, resp.Version)
	assert.Equal(t, memberID, resp.Leader)
	assert.Equal(t, raftTerm, resp.RaftTerm)
	assert.Equal(t, uint64(42), resp.RaftIndex)
	require.NotNil(t, resp.Header)
	assert.Equal(t, int64(42), resp.Header.Revision)
}

func TestMaintenance_Alarm_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	m := NewMaintenanceServer(&stubRepo{rev: 7})

	resp, err := m.Alarm(context.Background(), &etcdserverpb.AlarmRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp.Header)
	assert.Equal(t, int64(7), resp.Header.Revision)
	assert.Empty(t, resp.Alarms)
}

func TestCluster_MemberList(t *testing.T) {
	t.Parallel()

	c := NewClusterServer(&stubRepo{rev: 3})

	resp, err := c.MemberList(context.Background(), &etcdserverpb.MemberListRequest{})
	require.NoError(t, err)

	require.Len(t, resp.Members, 1)
	assert.Equal(t, memberID, resp.Members[0].ID)
	assert.Equal(t, "elara", resp.Members[0].Name)
	assert.NotEmpty(t, resp.Members[0].ClientURLs)
	assert.Equal(t, int64(3), resp.Header.Revision)
}
