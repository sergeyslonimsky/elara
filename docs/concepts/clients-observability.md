# Clients & Observability

"Clients" in Elara means consumers of the **etcd-compatible gRPC API** — the
services reading configs over the etcd wire protocol. UI and ConnectRPC users
are not clients.

## Client identification

When a gRPC connection is established, Elara captures a `ConnectionInfo` from
gRPC metadata (`internal/transport/grpc/stats_handler.go`,
`internal/domain/client.go`):

- `UserAgent` — the standard gRPC `user-agent`.
- `ClientName` / `ClientVersion` — from the `x-client-name` and
  `x-client-version` headers.
- `K8sNamespace` / `K8sPod` / `K8sNode` — from `x-client-k8s-namespace`,
  `x-client-k8s-pod`, `x-client-k8s-node`.
- `InstanceID` — from `x-client-instance-id`.
- `PeerAddress` — the connecting address.

Those `x-client-*` headers are supplied by the client (typically wired from the
Kubernetes downward API); when absent, the fields are simply empty.

## The connected-clients monitor

The in-memory monitor registry (`internal/service/monitor/registry.go`) tracks,
per connected client: request counts by RPC method, error count, last-activity
time, and the list of **active watches** (each with its key range, start
revision, and flags). It also keeps a small ring buffer of recent per-RPC events
and publishes changes so the UI can update live.

- The **Active** view lists clients that are currently connected
  (`DisconnectedAt == nil`).
- The **History** view shows clients that have since disconnected.

The live registry, the active-watch list, and the per-RPC ring buffer are
in-memory only and are lost on restart. **Connection history is persisted** to
bbolt and survives restarts; how much is kept is governed by
`CLIENT_HISTORY_MAX_RECORDS` and `CLIENT_HISTORY_MAX_AGE` — see
[Configuration](../deployment/configuration.md).

## Metrics and tracing

Both are opt-in and off by default so a bare instance boots without a
Prometheus Operator or OTel collector nearby. See
[Deployment → Configuration reference](../deployment/configuration.md#observability)
for the env vars, and
[Deployment → Kubernetes](../deployment/kubernetes.md) for wiring a
`ServiceMonitor`/OTLP endpoint in the Helm chart.
