package etcdv3

// Fixed cluster/member identifiers for etcd client compatibility.
// We are a single-instance non-raft implementation, so these are stable stubs.
const (
	clusterID uint64 = 0xe1_61_72_61_0001 // "elara" magic
	memberID  uint64 = 0xe1_61_72_61_0002
	raftTerm  uint64 = 1
)

// etcdVersion is reported via Maintenance.Status for clients that gate on version.
const etcdVersion = "3.5.0"
