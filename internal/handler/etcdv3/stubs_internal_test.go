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

	assert.Equal(t, etcdVersion, resp.GetVersion())
	assert.Equal(t, memberID, resp.GetLeader())
	assert.Equal(t, raftTerm, resp.GetRaftTerm())
	assert.Equal(t, uint64(42), resp.GetRaftIndex())
	require.NotNil(t, resp.GetHeader())
	assert.Equal(t, int64(42), resp.GetHeader().GetRevision())
}

func TestMaintenance_Alarm_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	m := NewMaintenanceServer(&stubRepo{rev: 7})

	resp, err := m.Alarm(context.Background(), &etcdserverpb.AlarmRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp.GetHeader())
	assert.Equal(t, int64(7), resp.GetHeader().GetRevision())
	assert.Empty(t, resp.GetAlarms())
}

func TestCluster_MemberList(t *testing.T) {
	t.Parallel()

	c := NewClusterServer(&stubRepo{rev: 3})

	resp, err := c.MemberList(context.Background(), &etcdserverpb.MemberListRequest{})
	require.NoError(t, err)

	require.Len(t, resp.GetMembers(), 1)
	assert.Equal(t, memberID, resp.GetMembers()[0].GetID())
	assert.Equal(t, "elara", resp.GetMembers()[0].GetName())
	assert.NotEmpty(t, resp.GetMembers()[0].GetClientURLs())
	assert.Equal(t, int64(3), resp.GetHeader().GetRevision())
}
