package demo

import (
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

const (
	methodRange = "Range"
	methodWatch = "Watch"
)

// sampleClient describes one simulated etcd consumer, as if a Kubernetes pod
// had connected and were watching a config key.
type sampleClient struct {
	info       domain.ConnectionInfo
	watchKey   string // etcd-encoded "/{namespace}{path}"
	watchRev   int64
	rangeCalls int
}

var sampleClients = []sampleClient{
	{
		info: domain.ConnectionInfo{
			PeerAddress:   "10.42.1.13:52344",
			UserAgent:     "grpc-go/1.68.0",
			ClientName:    "checkout-service",
			ClientVersion: "2.4.1",
			K8sNamespace:  "shop",
			K8sPod:        "checkout-service-7d9c8b6f5-x2kql",
			K8sNode:       "ip-10-42-1-13.ec2.internal",
			InstanceID:    "checkout-7d9c8b6f5-x2kql",
		},
		watchKey:   "/production/api/limits.json",
		watchRev:   7,
		rangeCalls: 12,
	},
	{
		info: domain.ConnectionInfo{
			PeerAddress:   "10.42.2.31:41022",
			UserAgent:     "grpc-go/1.68.0",
			ClientName:    "payment-service",
			ClientVersion: "1.9.0",
			K8sNamespace:  "shop",
			K8sPod:        "payment-service-5f7b9c4d8-nq7rt",
			K8sNode:       "ip-10-42-2-31.ec2.internal",
			InstanceID:    "payment-5f7b9c4d8-nq7rt",
		},
		watchKey:   "/production/api/timeouts.json",
		watchRev:   7,
		rangeCalls: 8,
	},
	{
		info: domain.ConnectionInfo{
			PeerAddress:   "10.42.3.7:39880",
			UserAgent:     "grpc-node/1.10.0",
			ClientName:    "web-frontend",
			ClientVersion: "3.1.2",
			K8sNamespace:  "web",
			K8sPod:        "web-frontend-6c4d7f9b8-hm5wp",
			K8sNode:       "ip-10-42-3-7.ec2.internal",
			InstanceID:    "web-frontend-6c4d7f9b8-hm5wp",
		},
		watchKey:   "/staging/features/flags.json",
		watchRev:   4,
		rangeCalls: 21,
	},
	{
		info: domain.ConnectionInfo{
			PeerAddress:   "10.42.4.19:57611",
			UserAgent:     "grpc-python/1.62.0",
			ClientName:    "batch-worker",
			ClientVersion: "0.8.3",
			K8sNamespace:  "jobs",
			K8sPod:        "batch-worker-84fd6c7b9-zt4vs",
			K8sNode:       "ip-10-42-4-19.ec2.internal",
			InstanceID:    "batch-worker-84fd6c7b9-zt4vs",
		},
		watchKey:   "/dev/features/flags.json",
		watchRev:   3,
		rangeCalls: 5,
	},
}

// seedClients registers the simulated clients in the connected-clients monitor
// so the UI shows k8s-aware consumers reading specific keys. These snapshots are
// in-memory only and are lost on restart, hence they are re-injected every boot.
func seedClients(registry clientRegistry) {
	now := time.Now()

	for i := range sampleClients {
		c := &sampleClients[i]
		connID := registry.RegisterConnection(c.info)

		registry.RegisterWatch(connID, domain.ActiveWatch{
			WatchID:        int64(i + 1),
			StartKey:       c.watchKey,
			StartRevision:  c.watchRev,
			CreatedAt:      now,
			ProgressNotify: true,
		})

		for range c.rangeCalls {
			registry.RecordRequest(connID, methodRange, c.watchKey, c.watchRev, 2*time.Millisecond, nil)
		}

		registry.RecordRequest(connID, methodWatch, c.watchKey, c.watchRev, time.Millisecond, nil)
	}
}
