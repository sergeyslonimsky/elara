package service

import (
	coregrpc "github.com/sergeyslonimsky/core/grpc"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"google.golang.org/grpc"

	"github.com/sergeyslonimsky/elara/internal/handler/etcdv3"
	grpctransport "github.com/sergeyslonimsky/elara/internal/transport/grpc"
)

type EtcdHandlers struct {
	KV          *etcdv3.KVServer
	Watch       *etcdv3.WatchServer
	Maintenance *etcdv3.MaintenanceServer
	Cluster     *etcdv3.ClusterServer
}

func NewEtcdHandlers(adapters *Adapters) *EtcdHandlers {
	// WatchServer integrates with the connected-clients monitor: each create/cancel
	// adjusts the active-watches counter on the originating connection. The conn
	// ID is stashed by the gRPC stats.Handler at TagConn time.
	watchServer := etcdv3.NewWatchServer(adapters.ConfigRepo, adapters.Watch).
		WithTracker(adapters.ClientRegistry, grpctransport.ConnIDFromContext)

	return &EtcdHandlers{
		KV:          etcdv3.NewKVServer(adapters.ConfigRepo, adapters.Watch),
		Watch:       watchServer,
		Maintenance: etcdv3.NewMaintenanceServer(adapters.ConfigRepo),
		Cluster:     etcdv3.NewClusterServer(adapters.ConfigRepo),
	}
}

// EtcdRoutes registers all etcd v3 gRPC services on the given gRPC server.
func EtcdRoutes(server *coregrpc.Server, handlers *EtcdHandlers) {
	server.Mount(func(gs *grpc.Server) {
		etcdserverpb.RegisterKVServer(gs, handlers.KV)
		etcdserverpb.RegisterWatchServer(gs, handlers.Watch)
		etcdserverpb.RegisterMaintenanceServer(gs, handlers.Maintenance)
		etcdserverpb.RegisterClusterServer(gs, handlers.Cluster)
	})
}
