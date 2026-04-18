package etcdv3

import (
	"context"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
)

type ClusterServer struct {
	etcdserverpb.UnimplementedClusterServer

	repo MaintenanceRepo
}

func NewClusterServer(repo MaintenanceRepo) *ClusterServer {
	return &ClusterServer{repo: repo}
}

func (s *ClusterServer) MemberList(
	ctx context.Context,
	_ *etcdserverpb.MemberListRequest,
) (*etcdserverpb.MemberListResponse, error) {
	rev, _ := s.repo.CurrentRevisionValue(ctx)

	return &etcdserverpb.MemberListResponse{
		Header: newHeader(rev),
		Members: []*etcdserverpb.Member{
			{
				ID:         memberID,
				Name:       "elara",
				PeerURLs:   []string{},
				ClientURLs: []string{"http://localhost:2379"},
			},
		},
	}, nil
}
