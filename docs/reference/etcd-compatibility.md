# etcd API compatibility

Elara serves the etcd v3 gRPC API on port `2379` (`internal/handler/etcdv3/`).
Any etcd v3 client — `etcdctl`, `clientv3`, or an SDK in another language —
connects unchanged, provided it only uses the RPCs below.

## Key encoding

A config is addressable as `/{namespace}{path}`, where `path` keeps its
leading slash:

```text
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
| `Txn` | ✅ | Atomic: the compares and the branch they select run in one transaction, so compare-and-swap is safe under concurrent writers and a multi-op branch is all-or-nothing. A `Txn` that only reads is served from one snapshot — every range sees the same revision. Watch notifications are published after the commit, never for a rolled-back write. A `Txn` with a write in either branch takes the writer path even when the branch it runs only reads. |
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
| **Auth `Authenticate`** | ✅ Supported | Log in with a token's username as the user name and the raw token as the password (`etcdctl --user`, `clientv3.Config{Username, Password}`). The returned auth token is sent as `token` metadata — see [Sessions & Tokens](../auth/sessions-tokens.md#using-a-token-against-the-etcd-grpc-api). |
| **Auth management** (etcd's own `AuthEnable`/`UserAdd`/`RoleGrantPermission`/…) | ❌ Not applicable | Elara has its own token-based auth for this port — see [Client Auth (etcd)](../auth/client-auth.md) — not etcd's built-in RBAC. Don't confuse the two. |

## Lease

| RPC | Support | Notes |
|---|---|---|
| `LeaseGrant` | ✅ Supported | A requested TTL is clamped to the configured bounds and the granted TTL is returned, as in etcd. Asking for an ID that is taken fails with `FailedPrecondition`. |
| `LeaseKeepAlive` | ✅ Supported | A lease that will not be renewed — unknown or already expired — is answered with `TTL 0` rather than a stream error, which is the contract `clientv3` relies on to close its keep-alive channel. |
| `LeaseRevoke` | ✅ Supported | Deletes every key the lease held in a single revision, and publishes one watch batch for them. |
| `LeaseTimeToLive` | ✅ Supported | Reports remaining and granted TTL; `-1` means there is no such lease. With `keys`, the attached keys are returned, gated by the token's namespace scope. |
| `LeaseLeases` | ✅ Supported | Lease IDs are not filtered by namespace: an ID carries no key material, and every operation that can act on one checks the scope of the keys it holds. |

Keys attached to a lease are deleted by a background sweep when the lease
expires. The sweep reads expiry from the store rather than from an in-process
timer, so a restart loses nothing.

**Renewals are throttled.** `KeepAlive` answers immediately and writes the new
expiry only once it has drifted past a fraction of the TTL
(`lease.checkpoint.threshold`, default 0.5). The cost is bounded staleness: after
an abrupt process death, a lease can expire up to that fraction of its TTL
earlier than the client was last promised. This is the same trade etcd makes when
lease checkpointing is off. The benefit is measurable — see
[Performance](performance.md).

Lock and leader-election recipes from `clientv3/concurrency` work against Elara:
`Session`, `Mutex`, and `Election` (including a leader's lease expiring and
leadership moving to a waiting candidate) are covered by the integration suite.

## Authentication and error mapping

## Where the write path diverges

A key written through `Put` is stored differently from the same key written
through the ConnectRPC `ConfigService`:

- **Content is stored byte-for-byte.** The normalization the management path
  applies (re-marshaling, canonical formatting) is deliberately skipped, so a
  value round-trips exactly as written. etcd values are frequently not
  structured config at all — distributed locks, leader-election payloads — and
  rewriting them would break clients that compare bytes.
- **Path validation is skipped.** A key written over etcd may carry a path the
  ConnectRPC API would reject.

Schema validation is the one management-path rule that *does* apply — see the
`Put` row above.

Requests are authenticated with a service token — either through etcd's own
login (`Auth.Authenticate` with the token's username and the raw token, then
`token` metadata), or as `authorization: Bearer` metadata (or unauthenticated
if `client.auth.enabled=false`) — see
[Sessions & Tokens → Tokens](../auth/sessions-tokens.md#tokens-service-credentials-for-etcd-clients).
Authentication errors use etcd's own messages so stock clients react to them
correctly: a failed login is `InvalidArgument` `etcdserver: authentication
failed, invalid user ID or password`; an invalid or expired token on a call is
`Unauthenticated` `etcdserver: invalid auth token` (which makes `clientv3`
log in again on its own); missing credentials are `Unauthenticated`. Beyond
authentication, the etcd wire protocol returns plain gRPC status codes:
`PermissionDenied` for a namespace or role check failure,
`FailedPrecondition` for a locked config/namespace, and `InvalidArgument` for a
value that fails the namespace's attached JSON Schema. See
[Reference → Errors catalog](errors-catalog.md) for the full domain-error
mapping.
