package etcdv3

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.uber.org/mock/gomock"

	mocketcd "github.com/sergeyslonimsky/elara/internal/handler/etcdv3/mocks"
)

func TestMaintenanceServer_Status(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocketcd.NewMockMaintenanceRepo(ctrl)
	s := NewMaintenanceServer(repo)

	repo.EXPECT().CurrentRevisionValue(gomock.Any()).Return(int64(200), nil)

	resp, err := s.Status(context.Background(), &etcdserverpb.StatusRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(200), resp.GetHeader().GetRevision())
	assert.Equal(t, uint64(200), resp.GetRaftIndex())
	assert.Equal(t, etcdVersion, resp.GetVersion())
	assert.Equal(t, memberID, resp.GetLeader())
}

func TestMaintenanceServer_Alarm(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocketcd.NewMockMaintenanceRepo(ctrl)
	s := NewMaintenanceServer(repo)

	repo.EXPECT().CurrentRevisionValue(gomock.Any()).Return(int64(42), nil)

	resp, err := s.Alarm(context.Background(), &etcdserverpb.AlarmRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(42), resp.GetHeader().GetRevision())
	assert.Empty(t, resp.GetAlarms())
}
