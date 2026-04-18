package etcdv3

import (
	"context"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
)

// MaintenanceRepo exposes the minimum needed for Status responses.
type MaintenanceRepo interface {
	CurrentRevisionValue(ctx context.Context) (int64, error)
}

type MaintenanceServer struct {
	etcdserverpb.UnimplementedMaintenanceServer

	repo MaintenanceRepo
}

func NewMaintenanceServer(repo MaintenanceRepo) *MaintenanceServer {
	return &MaintenanceServer{repo: repo}
}

func (s *MaintenanceServer) Status(
	ctx context.Context,
	_ *etcdserverpb.StatusRequest,
) (*etcdserverpb.StatusResponse, error) {
	rev, _ := s.repo.CurrentRevisionValue(ctx)

	return &etcdserverpb.StatusResponse{
		Header:    newHeader(rev),
		Version:   etcdVersion,
		DbSize:    0, // not tracked — bbolt file size could be exposed later
		Leader:    memberID,
		RaftIndex: uint64(rev),
		RaftTerm:  raftTerm,
	}, nil
}

func (s *MaintenanceServer) Alarm(
	ctx context.Context,
	_ *etcdserverpb.AlarmRequest,
) (*etcdserverpb.AlarmResponse, error) {
	rev, _ := s.repo.CurrentRevisionValue(ctx)

	return &etcdserverpb.AlarmResponse{Header: newHeader(rev)}, nil
}
