# etcd API compatibility

Elara serves the etcd v3 gRPC API on port `2379` (`internal/handler/etcdv3/`).
Any etcd v3 client — `etcdctl`, `clientv3`, or an SDK in another language —
connects unchanged, provided it only uses the RPCs below.

## Key encoding

A config is addressable as `/{namespace}{path}`, where `path` keeps its
leading slash:

```
namespace "prod" + path "/services/api.yaml"  →  /prod/services/api.yaml
```

The first path segment after the leading `/` is the namespace; everything
from the second `/` onward is the config path
(`internal/handler/etcdv3/keyencoding.go`). See
[Namespaces & Configs](../concepts/namespaces-configs.md#configs) for the
underlying domain fields this maps to.

## Supported RPCs

### KV (`kv_server.go`)

| RPC | Support | Notes |
|---|---|---|
| `Range` | ✅ Full | Prefix ranges (`range_end`) and full-keyspace scans work; historical reads by revision are supported. |
| `Put` | ✅ | Writes a config. **Runs JSON Schema validation** against the namespace's attached schema — a violation is rejected with `InvalidArgument` and nothing is stored (see [Schema Validation](../concepts/schema-validation.md)). Rejected with `FailedPrecondition` if the target config or its namespace is locked. `ignore_value` is rejected with `Unimplemented`. |
| `DeleteRange` | ✅ | Supports `PrevKv` to return deleted values. |
| `Txn` | ⚠️ Partial | Compare-and-swap style transactions work, but the transaction is **not fully atomic** with the rest of the write path yet — a multi-op `Txn` is not guaranteed all-or-nothing under concurrent writes the way real etcd's is. Don't rely on it for correctness-critical multi-key transactions today. |
| `Compact` | ⚠️ No-op | Accepted and returns success, but Elara never truncates history — there is nothing to compact. Safe to call (e.g. from a client library that compacts periodically); it just does nothing. |

### Watch (`watch_server.go`)

| RPC | Support | Notes |
|---|---|---|
| `Watch` | ✅ Full | Bidirectional streaming, `CreateRequest`/`CancelRequest`, `StartRevision` for resumable watches (history replay then live), `PrevKv`. See [Watch & Revisions](../concepts/watch-revisions.md). |

### Maintenance (`maintenance_server.go`)

| RPC | Support | Notes |
|---|---|---|
| `Status` | ⚠️ Synthetic | Returns a response shaped like real etcd's (current revision, a static version string, a fixed member/leader id) so clients that check it don't break, but there's no raft cluster behind it — the numbers describe a single-node stand-in. |
| `Alarm` | ⚠️ Always empty | Accepted; always returns no alarms. Elara has no equivalent of etcd's NOSPACE/CORRUPT alarms. |
| `Snapshot` | ❌ Unimplemented | No online backup RPC. See [Troubleshooting → Backup & Restore](troubleshooting.md#backup-restore). |
| `Defragment`, `Hash`, `HashKV`, `MoveLeader`, `Downgrade` | ❌ Unimplemented | Not applicable to a single bbolt-backed instance. |

### Cluster (`cluster_server.go`)

| RPC | Support | Notes |
|---|---|---|
| `MemberList` | ⚠️ Synthetic | Returns a single synthetic member so clients that call it (some SDKs do, for endpoint discovery) don't break. |
| `MemberAdd`, `MemberRemove`, `MemberUpdate`, `MemberPromote` | ❌ Unimplemented | Elara is single-instance; there is no cluster to reconfigure. |

## Explicitly unsupported

| API | Status | Why |
|---|---|---|
| **Lease** (`LeaseGrant`, `LeaseKeepAlive`, `LeaseRevoke`, `LeaseTimeToLive`, `LeaseLeases`) | ❌ Not registered at all | No `LeaseServer` is wired up — key TTLs and lease-based locks/elections that depend on leases (e.g. etcd `concurrency` package election recipes) will not work against Elara. |
| **Auth API** (etcd's own `AuthEnable`/`UserAdd`/`RoleGrantPermission`/…) | ❌ Not applicable | Elara has its own token-based auth for this port — see [Client Auth (etcd)](../auth/client-auth.md) — not etcd's built-in RBAC. Don't confuse the two. |

## Authentication and error mapping

Requests are authenticated via a Bearer service token in gRPC metadata (or
unauthenticated if `client.auth.enabled=false`) — see
[Sessions & Tokens → Tokens](../auth/sessions-tokens.md#tokens-service-credentials-for-etcd-clients).
Unlike the ConnectRPC API's structured error codes, the etcd wire protocol
returns plain gRPC status codes: `Unauthenticated` for a missing/invalid/expired
token, `PermissionDenied` for a namespace or role check failure,
`FailedPrecondition` for a locked config/namespace, and `InvalidArgument` for a
value that fails the namespace's attached JSON Schema. See
[Reference → Errors catalog](errors-catalog.md) for the full domain-error
mapping.
