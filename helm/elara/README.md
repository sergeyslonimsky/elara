# Elara Helm Chart

Helm chart for **Elara** — a configuration management service with a Web UI,
a ConnectRPC API, and an etcd-compatible gRPC API. Storage is backed by an
embedded bbolt database (single instance); raft-based HA and additional
storage backends (PostgreSQL, S3) are on the roadmap.

Chart source: <https://github.com/sergeyslonimsky/elara/tree/master/helm>

## TL;DR

```bash
helm repo add elara https://sergeyslonimsky.github.io/elara
helm install elara elara/elara
```

Once the gh-pages chart repository is published, the commands above are the
only install instructions users need. Until then, install directly from a
checkout:

```bash
helm install elara ./helm/elara
```

## Prerequisites

| Requirement           | Notes                                                           |
| --------------------- | --------------------------------------------------------------- |
| Kubernetes ≥ 1.25     | Enforced by `Chart.yaml.kubeVersion`                            |
| Helm ≥ 3.10           | For `values.schema.json` validation                             |
| A default StorageClass | bbolt persistence uses `volumeClaimTemplates` with RWO          |
| *(optional)* Prometheus Operator | Required only when enabling `ServiceMonitor`         |
| *(optional)* cert-manager        | For automatic TLS on the Ingress                     |

## Quick install examples

### Default (minimum viable)
```bash
helm install elara ./helm/elara
```
Creates a single-replica StatefulSet with an 2 Gi RWO PVC, exposes an
internal ClusterIP service on ports 8080 (HTTP/UI/ConnectRPC) and 2379
(etcd gRPC). No Ingress, no metrics, no tracing.

### With HTTP ingress and TLS
```bash
helm install elara ./helm/elara \
  --set ingress.enabled=true \
  --set ingress.className=nginx \
  --set ingress.hosts[0].host=elara.example.com \
  --set ingress.tls[0].hosts[0]=elara.example.com \
  --set ingress.tls[0].secretName=elara-tls
```
The Ingress exposes the HTTP/ConnectRPC port only. The etcd-compatible
gRPC port (2379) is intentionally not routed through a standard HTTP
Ingress — see *gRPC exposure* below.

### With Prometheus ServiceMonitor
```bash
helm install elara ./helm/elara \
  --set metrics.enabled=true \
  --set metrics.serviceMonitor.enabled=true \
  --set metrics.serviceMonitor.labels.release=kube-prometheus-stack
```

### With OTLP tracing
```bash
helm install elara ./helm/elara \
  --set tracing.enabled=true \
  --set tracing.otlpEndpoint=http://otel-collector.observability:4318
```

### With production-grade resources + persistence
```yaml
# values-prod.yaml
resources:
  requests: { cpu: 250m, memory: 256Mi }
  limits:   { cpu: "2",  memory: 1Gi   }
persistence:
  size: 50Gi
  storageClassName: ssd
metrics:
  enabled: true
  serviceMonitor:
    enabled: true
tracing:
  enabled: true
  otlpEndpoint: http://otel-collector.observability:4318
ingress:
  enabled: true
  className: nginx
  hosts:
    - host: elara.example.com
      paths:
        - { path: /, pathType: Prefix, port: http }
  tls:
    - secretName: elara-tls
      hosts: [elara.example.com]
```
```bash
helm install elara ./helm/elara -f values-prod.yaml
```

## Values reference

Full list with descriptions lives in [`values.yaml`](values.yaml). Key
sections:

| Key                       | Default                         | Purpose                                                |
| ------------------------- |---------------------------------| ------------------------------------------------------ |
| `image.repository`        | `ghcr.io/sergeyslonimsky/elara` | Container image                                        |
| `image.tag`               | Chart `appVersion`              | Pin a specific version                                 |
| `image.digest`            | `""`                            | Overrides `tag` for immutable deploys                  |
| `replicaCount`            | `1`                             | **Invariant**: schema pins to `1` until raft HA lands  |
| `config.http.port`        | `8080`                          | HTTP/2, ConnectRPC, Web UI                             |
| `config.http.writeTimeout`| `24h`                           | Server-streaming RPCs need a long write timeout        |
| `config.grpc.port`        | `2379`                          | etcd-compatible gRPC API                               |
| `config.clients.*`        | see `values.yaml`               | Connected-clients monitor tuning                       |
| `storage.type`            | `bbolt`                         | Schema enum: `[bbolt]` today                           |
| `storage.bbolt.path`      | `/var/lib/elara`                | Directory inside the PVC mount                         |
| `persistence.size`        | `2Gi`                           | PVC size via `volumeClaimTemplates`                    |
| `persistence.accessMode`  | `ReadWriteOnce`                 | bbolt requires exclusive lock                          |
| `metrics.enabled`         | `false`                         | Exposes `/metrics` on the HTTP port                    |
| `metrics.serviceMonitor.enabled` | `false`                         | Requires Prometheus Operator CRDs                      |
| `tracing.enabled`         | `false`                         | OTLP push                                              |
| `tracing.otlpEndpoint`    | `""`                            | Required when `tracing.enabled=true`                   |
| `config.log.level`        | `info`                          | One of: `debug`, `info`, `warn`, `error`               |
| `config.log.format`       | `json`                          | One of: `json`, `text`                                 |
| `config.log.noSource`     | `false`                         | Set `true` to omit source file/line from log entries   |
| `service.type`            | `ClusterIP`                     | `NodePort`/`LoadBalancer` supported                    |
| `ingress.enabled`         | `false`                         | Exposes HTTP port only                                 |
| `networkPolicy.enabled`   | `false`                         | Optional; CNI-dependent                                |

## How configuration reaches the service

`values.yaml` → `ConfigMap` (env vars) → service reads them through viper.

Viper uses the core library's `SetEnvKeyReplacer(".", "_")`, so every viper
key maps to an env var by **uppercasing and replacing dots with underscores
— camelCase tokens are not split**. For example:

| Config key (viper)             | Env var (ConfigMap)           |
| ------------------------------ | ----------------------------- |
| `http.frontend.port`           | `HTTP_FRONTEND_PORT`          |
| `http.frontend.readTimeout`    | `HTTP_FRONTEND_READTIMEOUT`   |
| `http.frontend.writeTimeout`   | `HTTP_FRONTEND_WRITETIMEOUT`  |
| `grpc.etcd.port`               | `GRPC_ETCD_PORT`              |
| `config.data.path`             | `CONFIG_DATA_PATH`            |
| `service.name`                 | `SERVICE_NAME`                |
| `metrics.enabled`              | `METRICS_ENABLED`             |
| `tracing.otlp.endpoint`        | `TRACING_OTLP_ENDPOINT`       |
| `clients.history.max_records`  | `CLIENTS_HISTORY_MAX_RECORDS` |
| `log.level`                    | `LOG_LEVEL`                   |
| `log.format`                   | `LOG_FORMAT`                  |
| `log.noSource`                 | `LOG_NOSOURCE`                |

Add extra env-vars via `extraEnv` or wire a Secret with `extraEnvFrom`.

## gRPC exposure

The etcd-compatible gRPC API on port 2379 is **not** routed through the
default HTTP Ingress. Common patterns:

1. **Cluster-internal only** (default): consume via the ClusterIP service.
   ```
   elara.{namespace}.svc.cluster.local:2379
   ```
2. **External via LoadBalancer**: set `service.type=LoadBalancer`; clients
   connect directly to the LB IP on port 2379.
3. **External via gRPC-aware ingress**: create a dedicated Ingress or
   Gateway resource pointing at the `grpc` port. Example with nginx-ingress:
   ```yaml
   metadata:
     annotations:
       nginx.ingress.kubernetes.io/backend-protocol: GRPC
   spec:
     rules:
       - host: etcd.elara.example.com
         http:
           paths:
             - path: /
               pathType: Prefix
               backend:
                 service:
                   name: elara
                   port: { name: grpc }
   ```
   Not shipped in the chart by default — add it via `extraObjects` or a
   sibling manifest.

## Persistence notes

- The PVC is created via StatefulSet `volumeClaimTemplates`. `helm uninstall`
  does **not** delete the PVC — delete it manually when you want to wipe
  data.
- For existing storage, set `persistence.existingClaim` and the chart will
  mount that PVC instead of creating one. This works only with
  `replicaCount: 1`.
- `persistence.enabled: false` uses `emptyDir` — data is lost on pod
  restart. Use only for dev or CI.

## Roadmap

| Feature                                     | Impact on chart                                              |
| ------------------------------------------- | ------------------------------------------------------------ |
| Raft-based HA                               | Ships HPA template, relaxes `replicaCount` schema, adds PDB  |
| PostgreSQL storage backend                  | Unlocks `storage.postgres.*` + migrations init container     |
| S3 storage backend                          | Unlocks `storage.s3.*`                                       |
| Official gh-pages chart hosting             | `helm repo add elara …` one-liner                            |

## Publishing (maintainers)

Once migrated to GitHub, the chart and image will be published by the
repository's GitHub Actions workflow using [`helm/chart-releaser-action`]
for the chart (→ gh-pages branch, served via GitHub Pages) and
`docker/build-push-action` for the image (→ `ghcr.io/sergeyslonimsky/elara`).

[`helm/chart-releaser-action`]: https://github.com/helm/chart-releaser-action

## Testing

```bash
helm lint ./helm/elara
helm template elara ./helm/elara                     # render templates
helm install elara ./helm/elara --dry-run --debug    # schema + template dry run
helm test elara                                # in-cluster smoke test
```

## License

MIT. See the repository root for the full license text.
