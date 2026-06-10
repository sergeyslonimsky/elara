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

func TestClusterServer_MemberList(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocketcd.NewMockMaintenanceRepo(ctrl)
	s := NewClusterServer(repo)

	repo.EXPECT().CurrentRevisionValue(gomock.Any()).Return(int64(100), nil)

	resp, err := s.MemberList(context.Background(), &etcdserverpb.MemberListRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(100), resp.Header.Revision)
	require.Len(t, resp.Members, 1)
	assert.Equal(t, "elara", resp.Members[0].Name)
}
