package integration

import (
	"context"
	"net"
	"testing"
	"time"

	coregrpc "github.com/sergeyslonimsky/core/grpc"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/sergeyslonimsky/elara/internal/di/service"
)

// StartEtcd boots a real etcd-compatible gRPC server (KV/Watch/Maintenance/
// Cluster/Lease) backed by s's adapters — the same bbolt store already wired
// for the HTTP suite — on an ephemeral port, and returns a connected
// clientv3.Client.
//
// No auth interceptor is installed: this exercises unauthenticated
// wire-protocol behavior only.
//
// Leases are served but do not expire here: the sweep is a separate worker, so
// a test that waits for a TTL to run out must also call StartLeaseExpirer.
func (s *Suite) StartEtcd(t *testing.T) *clientv3.Client {
	t.Helper()

	handlers := service.NewEtcdHandlers(s.Adapters, s.Services)

	ctx, cancel := context.WithCancel(t.Context())

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := coregrpc.NewServer(coregrpc.Config{}, coregrpc.WithListener(listener))
	service.EtcdRoutes(grpcServer, handlers)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = grpcServer.Run(ctx)
	}()

	t.Cleanup(func() {
		cancel()
		<-done
	})

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{listener.Addr().String()},
		DialTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = cli.Close() })

	return cli
}

// StartLeaseExpirer runs the lease expiry sweep for the duration of the test.
//
// In production this is a goroutine started at boot (cmd/service/main.go). Tests
// opt in because most of them have no expiring lease, and a sweep ticking under
// every suite would be work no assertion depends on.
//
// Its pace comes from the suite's config (testConfig sets a short interval, so a
// test waiting for an expiry waits a fraction of a second rather than the
// production second).
func (s *Suite) StartLeaseExpirer(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	expirer := s.Services.LeaseExpirer

	done := make(chan struct{})

	go func() {
		defer close(done)

		_ = expirer.Run(ctx)
	}()

	t.Cleanup(func() {
		cancel()
		<-done
	})
}
