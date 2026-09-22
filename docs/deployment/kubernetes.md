# Kubernetes (Helm)

Elara ships an official Helm chart. Storage is backed by an embedded bbolt
database (single instance); raft-based HA and additional storage backends
(PostgreSQL, S3) are on the roadmap.

Chart source: <https://github.com/sergeyslonimsky/elara/tree/master/helm>

## TL;DR

```bash
helm repo add elara https://sergeyslonimsky.github.io/elara
helm install elara elara/elara
```

Or install directly from a checkout:

```bash
helm install elara ./helm/elara
```

## Prerequisites

| Requirement | Notes |
|---|---|
| Kubernetes ≥ 1.25 | Enforced by `Chart.yaml.kubeVersion` |
| Helm ≥ 3.10 | For `values.schema.json` validation |
| A default StorageClass | bbolt persistence uses `volumeClaimTemplates` with RWO |
| *(optional)* Prometheus Operator | Required only when enabling `ServiceMonitor` |
| *(optional)* cert-manager | For automatic TLS on the Ingress |

## Quick install examples

### Default (minimum viable)

```bash
helm install elara ./helm/elara
```

Creates a single-replica StatefulSet with a 2 Gi RWO PVC, exposes an internal
ClusterIP service on ports 8080 (HTTP/UI/ConnectRPC) and 2379 (etcd gRPC). No
Ingress, no metrics, no tracing.

### With HTTP ingress and TLS

```bash
helm install elara ./helm/elara \
  --set ingress.enabled=true \
  --set ingress.className=nginx \
  --set ingress.hosts[0].host=elara.example.com \
  --set ingress.tls[0].hosts[0]=elara.example.com \
  --set ingress.tls[0].secretName=elara-tls
```

The Ingress exposes the HTTP/ConnectRPC port only. The etcd-compatible gRPC
port (2379) is intentionally not routed through a standard HTTP Ingress — see
[gRPC exposure](#grpc-exposure) below.

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

### With basic-auth (username + password)

```bash
helm install elara ./helm/elara \
  --set config.ui.auth.enabled=true \
  --set config.ui.auth.type=basic-auth \
  --set config.ui.auth.basicAuth.username=admin@example.com \
  --set config.ui.auth.basicAuth.password=ChangeMe123
```

The chart creates a `{release}-auth` Secret holding the sensitive values. The
admin is forced to change their password on first login — see
[Basic Auth Setup](../auth/basic.md).

For production, supply credentials from a pre-existing Secret (e.g. managed by
external-secrets or Vault) and omit the plaintext values:

```bash
helm install elara ./helm/elara \
  --set config.ui.auth.enabled=true \
  --set config.ui.auth.type=basic-auth \
  --set config.ui.auth.basicAuth.username=admin@example.com \
  --set config.ui.auth.existingSecret=my-elara-auth-secret
```

The existing Secret must contain `UI_AUTH_BASICAUTH_PASSWORD` (or
`UI_AUTH_OIDC_CLIENTSECRET` when `type=oidc`) — it is mounted with `envFrom`,
so any other key in it also reaches the service as an env var.

### With OIDC

```bash
helm install elara ./helm/elara \
  --set config.ui.auth.enabled=true \
  --set config.ui.auth.type=oidc \
  --set config.ui.auth.oidc.issuerUrl=https://accounts.google.com \
  --set config.ui.auth.oidc.clientId=MY_CLIENT_ID \
  --set config.ui.auth.oidc.clientSecret=MY_CLIENT_SECRET \
  --set "config.ui.auth.oidc.redirectUrl=https://elara.example.com/auth/callback" \
  --set config.ui.auth.oidc.adminEmail=admin@example.com
```

See [OIDC Setup](../auth/oidc.md) for how the admin bootstrap and anti-hijack
guard work.

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

Full list with descriptions lives in
[`values.yaml`](https://github.com/sergeyslonimsky/elara/blob/master/helm/elara/values.yaml).
Key sections:

| Key | Default | Purpose |
|---|---|---|
| `image.repository` | `ghcr.io/sergeyslonimsky/elara` | Container image |
| `image.tag` | Chart `appVersion` | Pin a specific version |
| `image.digest` | `""` | Overrides `tag` for immutable deploys |
| `replicaCount` | `1` | **Invariant**: schema pins to `1` until raft HA lands |
| `config.ui.server.port` | `8080` | HTTP/2, ConnectRPC, Web UI |
| `config.ui.server.writeTimeout` | `24h` | Server-streaming RPCs need a long write timeout |
| `config.ui.auth.enabled` | `false` | Enable authentication; all RPCs are public when false |
| `config.ui.auth.type` | `basic-auth` | `basic-auth`, `oidc`, or `none` |
| `config.ui.auth.basicAuth.username` | `""` | Bootstrap admin email (required for basic-auth) |
| `config.ui.auth.basicAuth.password` | `""` | Bootstrap admin password (basic-auth); stored in chart Secret |
| `config.ui.auth.oidc.issuerUrl` | `""` | OIDC issuer (e.g. `https://accounts.google.com`) |
| `config.ui.auth.oidc.clientId` | `""` | OIDC client ID |
| `config.ui.auth.oidc.clientSecret` | `""` | OIDC client secret; stored in chart Secret |
| `config.ui.auth.oidc.redirectUrl` | `""` | OIDC callback URL |
| `config.ui.auth.oidc.scopes` | `[]` | OIDC scopes; defaults to `[openid, email, profile]` |
| `config.ui.auth.oidc.adminEmail` | `""` | Bootstrap admin email; pre-provisioned as superadmin placeholder, claimed by first matching OIDC login (required for OIDC) |
| `config.ui.auth.session.secureCookie` | `true` | Add `Secure` flag to session cookie; disable only for HTTP dev |
| `config.ui.auth.existingSecret` | `""` | Use a pre-existing Secret instead of the chart-managed one |
| `config.client.etcd.port` | `2379` | etcd-compatible gRPC API |
| `config.client.auth.enabled` | `false` | Bearer-token auth on etcd gRPC + mounts the TokenService |
| `config.client.history.*` | see `values.yaml` | Connected-clients history tuning |
| `config.client.recentEvents.*` | see `values.yaml` | Recent-events ring buffer tuning |
| `storage.type` | `bbolt` | Schema enum: `[bbolt]` today |
| `storage.bbolt.path` | `/var/lib/elara` | Directory inside the PVC mount |
| `persistence.size` | `2Gi` | PVC size via `volumeClaimTemplates` |
| `persistence.accessMode` | `ReadWriteOnce` | bbolt requires exclusive lock |
| `metrics.enabled` | `false` | Exposes `/metrics` on the HTTP port |
| `metrics.serviceMonitor.enabled` | `false` | Requires Prometheus Operator CRDs |
| `tracing.enabled` | `false` | OTLP push |
| `tracing.otlpEndpoint` | `""` | Required when `tracing.enabled=true` |
| `config.log.level` | `info` | One of: `debug`, `info`, `warn`, `error` |
| `config.log.format` | `json` | One of: `json`, `text` |
| `config.log.noSource` | `false` | Set `true` to omit source file/line from log entries |
| `service.type` | `ClusterIP` | `NodePort`/`LoadBalancer` supported |
| `ingress.enabled` | `false` | Exposes HTTP port only |
| `networkPolicy.enabled` | `false` | Optional; CNI-dependent |

## How configuration reaches the service

`values.yaml` → `ConfigMap` (env vars) → service reads them through Viper,
same mapping rule as everywhere else in Elara — see
[Configuration → Environment-variable naming](configuration.md#environment-variable-naming).
Sensitive values (`basicAuth.password`, `oidc.clientSecret`) go into a Secret
instead of the ConfigMap. Session lifetime is not a chart value: sessions are
opaque server-side ids with compile-time TTLs — see
[Sessions & Tokens](../auth/sessions-tokens.md). Add extra env-vars via `extraEnv` or
wire a Secret with `extraEnvFrom`.

## gRPC exposure

The etcd-compatible gRPC API on port 2379 is **not** routed through the
default HTTP Ingress. Common patterns:

1. **Cluster-internal only** (default): consume via the ClusterIP service.

   ```
   elara.{namespace}.svc.cluster.local:2379
   ```

2. **External via LoadBalancer**: set `service.type=LoadBalancer`; clients
   connect directly to the LB IP on port 2379.
3. **External via gRPC-aware ingress**: create a dedicated Ingress or Gateway
   resource pointing at the `grpc` port. Example with nginx-ingress:

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
  does **not** delete the PVC — delete it manually when you want to wipe data.
- For existing storage, set `persistence.existingClaim` and the chart will
  mount that PVC instead of creating one. This works only with
  `replicaCount: 1`.
- `persistence.enabled: false` uses `emptyDir` — data is lost on pod restart.
  Use only for dev or CI.

## Roadmap

| Feature | Impact on chart |
|---|---|
| Raft-based HA | Ships HPA template, relaxes `replicaCount` schema, adds PDB |
| PostgreSQL storage backend | Unlocks `storage.postgres.*` + migrations init container |
| S3 storage backend | Unlocks `storage.s3.*` |

## Testing

```bash
helm lint ./helm/elara
helm template elara ./helm/elara                     # render templates
helm install elara ./helm/elara --dry-run --debug    # schema + template dry run
helm test elara                                # in-cluster smoke test
```
