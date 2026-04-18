<p align="center">
  <img src="./logo.svg" alt="Elara" width="128" height="128"/>
</p>

<h1 align="center">Elara</h1>

<p align="center">
  Configuration management service with a Web UI, a ConnectRPC API, and an etcd-compatible gRPC API.
</p>

Elara stores, edits, and serves application configuration. Operators use the
built-in Web UI for CRUD; services consume values through the same API
surface as etcd (drop-in for any etcd v3 client) or through a typed
ConnectRPC client. A single bbolt file holds all state with ACID
transactions and global revision tracking.

**Status:** early, pre-1.0. Single-instance bbolt today; raft-based HA and
pluggable storage backends (PostgreSQL, S3) are on the roadmap.

![Elara dashboard](./assets/dashboard_main.png)

## Features

- **Web UI** for browsing, creating, editing, and deleting configs across namespaces.
- **ConnectRPC API** (`elara.config.v1.ConfigService`, `elara.namespace.v1.NamespaceService`, …) — works from Go, TypeScript, Python, etc. with native clients.
- **etcd-compatible gRPC API** on port 2379 (`KV`, `Watch`, `Maintenance`, `Cluster`) — connect with `etcdctl` or any etcd v3 SDK.
- **Config history** — every version stored, retrievable by revision.
- **Global revision counter** — monotonic, etcd-style semantics.
- **Format-aware validation** for JSON and YAML; pass-through for everything else (ini, toml, plain text).
- **Single bbolt file storage** — ACID transactions, no external DB required.
- **Observability** — optional Prometheus `/metrics` and OTLP tracing.
- **Kube-native Helm chart** with StatefulSet, ServiceMonitor, NetworkPolicy, JSON Schema validation, and a smoke test.

## Quick start

Run locally with Docker:

```bash
docker run --rm -p 8080:8080 -p 2379:2379 ghcr.io/sergeyslonimsky/elara:latest
```

Open <http://localhost:8080> for the UI. Talk to the etcd-compatible API at
`localhost:2379`. Jump to [Deploying to Kubernetes](#deploying-to-kubernetes)
for the Helm path.

## Usage

### Web UI

The UI (served on the HTTP port, port 8080 by default) covers the full
operator workflow:

- **Dashboard** — cluster-wide KPIs (total namespaces, configs, active
  clients, current global revision) plus the last 20 config changes and a
  per-namespace config count.
- **Configs** — directory-style browser across folders/files, per-namespace.
  Create, edit (format-aware: JSON / YAML / raw), copy, delete, and view
  version history. Every edit bumps the global revision.
- **Namespaces** — CRUD for namespaces (logical grouping of configs).
  Deletion is blocked while the namespace still has configs.
- **Clients** — live list of connected etcd-compatible clients, with
  recent events and basic history.

### etcd-compatible CLI

Any etcd v3 client works. Example with `etcdctl`:

```bash
export ETCDCTL_API=3
export ETCDCTL_ENDPOINTS=localhost:2379

# Write a config (namespace = prefix segment, path = key)
etcdctl put /default/services/billing/config.yaml "$(cat config.yaml)"

# Read it back
etcdctl get /default/services/billing/config.yaml

# Watch a prefix for live updates
etcdctl watch --prefix /default/services/billing/

# Check endpoint health
etcdctl endpoint health
```

### ConnectRPC client (Go)

```go
import (
    "connectrpc.com/connect"
    "net/http"

    configv1 "github.com/sergeyslonimsky/elara/gen/elara/config/v1"
    "github.com/sergeyslonimsky/elara/gen/elara/config/v1/configv1connect"
)

client := configv1connect.NewConfigServiceClient(
    http.DefaultClient,
    "http://localhost:8080",
)

resp, _ := client.CreateConfig(ctx, connect.NewRequest(&configv1.CreateConfigRequest{
    Namespace: "default",
    Path:      "/services/billing/config.yaml",
    Content:   []byte("retries: 3\n"),
}))
```

### ConnectRPC client (TypeScript)

```ts
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { ConfigService } from "./gen/elara/config/v1/config_service_pb";

const client = createClient(
  ConfigService,
  createConnectTransport({ baseUrl: "http://localhost:8080" }),
);

await client.createConfig({
  namespace: "default",
  path: "/services/billing/config.yaml",
  content: new TextEncoder().encode("retries: 3\n"),
});
```

## Deploying to Kubernetes

The chart lives at [`helm/elara/`](helm/elara/) and is designed to be
production-ready by default: StatefulSet with `volumeClaimTemplates`,
non-root security context, JSON-Schema-validated values, optional
ServiceMonitor and NetworkPolicy, and a `helm test` smoke check.

### Install from the Helm repository

Once the GitHub Pages repo is published:

```bash
helm repo add elara https://sergeyslonimsky.github.io/elara
helm repo update

# default: single replica, 2Gi RWO PVC, ClusterIP service
helm install elara elara/elara --namespace elara --create-namespace
```

### Install from a checkout

```bash
helm install elara ./helm/elara --namespace elara --create-namespace
```

### Production values

```yaml
# values-prod.yaml
image:
  digest: sha256:…              # pin by digest, not tag, in prod

resources:
  requests: { cpu: 250m, memory: 256Mi }
  limits:   { cpu: "2",  memory: 1Gi   }

persistence:
  size: 50Gi
  storageClassName: ssd

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: elara.example.com
      paths: [ { path: /, pathType: Prefix, port: http } ]
  tls:
    - secretName: elara-tls
      hosts: [ elara.example.com ]

metrics:
  enabled: true
  serviceMonitor:
    enabled: true
    labels: { release: kube-prometheus-stack }

tracing:
  enabled: true
  otlpEndpoint: http://otel-collector.observability:4318
```

```bash
helm install elara elara/elara -f values-prod.yaml \
  --namespace elara --create-namespace
```

### Upgrade

```bash
helm upgrade elara elara/elara --namespace elara -f values-prod.yaml
```

Pods restart automatically on ConfigMap changes (via checksum annotation).
`helm.sh/resource-policy: keep` is NOT applied to the PVC, but because the
chart uses `volumeClaimTemplates`, `helm uninstall` leaves the PVC in
place regardless — data survives uninstall.

### Uninstall

```bash
helm uninstall elara --namespace elara

# Optional: drop the PVC too (destroys all stored configs)
kubectl delete pvc data-elara-0 --namespace elara
```

### Exposing the etcd-compatible gRPC port

The chart Ingress exposes only the HTTP / ConnectRPC / UI port (8080).
Port 2379 (etcd gRPC) is reachable cluster-internally over the ClusterIP
service by default. For external exposure, use `service.type: LoadBalancer`
or add a gRPC-aware Ingress — see the [chart
README](helm/elara/README.md#grpc-exposure).

### Invariants

- `replicaCount` is schema-pinned to `1` until raft-based HA is implemented.
  bbolt holds an exclusive file lock — more than one replica corrupts data.
  The schema will relax to `minimum: 1` when raft ships.
- `persistence.accessMode` is pinned to `ReadWriteOnce` for the same reason.
- `storage.type` currently accepts only `bbolt`; the enum will expand with
  future storage backends.

The full values reference, extensibility hooks, and examples live in
[`helm/elara/README.md`](helm/elara/README.md).

## Local development

```bash
make proto       # regenerate protobuf stubs
make test        # go test -race ./...
make lint        # golangci-lint
make format      # golines + gofumpt + gci
go run ./cmd/service
```

The UI is served embedded from `web/dist`; for live reload during frontend
work run `cd web && npm run dev` and hit <http://localhost:3000>.

## Architecture

```
Web UI (React)  ──┐
ConnectRPC client ┤──→  HTTP/2 server (:8080)  ──→  UseCases  ──→  Domain
etcdctl / grpc  ──────→  gRPC server  (:2379)  ──→  UseCases  ──→  Domain
                                                                       │
                                                           Adapter ────┘
                                                           (bbolt)
```

- **Handler** — ConnectRPC / etcd gRPC; proto ↔ domain conversion.
- **UseCase** — application logic; each usecase owns its minimal interface.
- **Domain** — pure entities, validation, errors; no infrastructure imports.
- **Adapter** — bbolt storage and in-memory watch pub/sub.

## Configuration

All config keys flow through Viper; environment variables override every
source. See the
[mapping table in the chart README](helm/elara/README.md#how-configuration-reaches-the-service)
for the full list.

Key defaults:

| Key                   | Env var                | Default          |
| --------------------- | ---------------------- | ---------------- |
| `http.frontend.port`  | `HTTP_FRONTEND_PORT`   | `8080`           |
| `grpc.etcd.port`      | `GRPC_ETCD_PORT`       | `2379`           |
| `config.data.path`    | `CONFIG_DATA_PATH`     | `./data`         |
| `metrics.enabled`     | `METRICS_ENABLED`      | `false`          |
| `tracing.enabled`     | `TRACING_ENABLED`      | `false`          |

## Contributing

PRs welcome. A few house rules:

- Go: `golines` (120 cols), `gofumpt`, `gci` (stdlib → default → `github.com/sergeyslonimsky/elara` prefix).
- Proto: `make proto` — `buf lint` and `buf breaking` run in CI.
- Tests: `go test -race` must pass.
- Keep changes focused; split unrelated refactors into separate PRs.

## License

[MIT](LICENSE).
