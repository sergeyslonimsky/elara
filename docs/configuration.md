# Configuration

Elara is configured entirely through [Viper](https://github.com/spf13/viper).
Values can come from a config file, but **environment variables override every
other source** — so a container deployment can be driven purely by env vars with
no config file at all.

## Environment-variable naming

Config keys are dotted (e.g. `ui.auth.basicAuth.username`). Viper is set up with
`AutomaticEnv()` and a `.` → `_` key replacer, then uppercases the whole key.
The mapping is therefore mechanical:

```
ui.auth.basicAuth.username   →   UI_AUTH_BASICAUTH_USERNAME
config.data.path             →   CONFIG_DATA_PATH
```

camelCase segments are **not** split on case — `basicAuth` becomes `BASICAUTH`,
not `BASIC_AUTH`. Every env var in the tables below was derived from this rule
and cross-checked against the key strings in `internal/di/config/`.

## Config file for local installs

When you run the `elara` binary directly (rather than the container image),
it auto-loads `~/.elara/config.yaml` if that file exists — no `APP_CONFIG_FILE_PATH`
needed. This is skipped entirely if you already set `APP_CONFIG_FILE_PATH` /
`APP_CONFIG_FILE_PATHS` yourself, and it never applies inside the container
image, which always configures itself through environment variables.

```yaml
# ~/.elara/config.yaml
ui:
  auth:
    type: basicAuth
    basicAuth:
      username: admin
      password: change-me
```

Environment variables still override anything set here.

## Reference

### Server & data

| Env Var | Config Key | Default | Description |
|---|---|---|---|
| `UI_SERVER_PORT` | `ui.server.port` | `8080` | Port for the HTTP/2 server that serves the Web UI, the ConnectRPC API, and (when enabled) `/metrics`. |
| `UI_SERVER_READTIMEOUT` | `ui.server.readTimeout` | `0` (no timeout) | Max duration for reading a request. Accepts Go durations (`5s`, `250ms`). |
| `UI_SERVER_WRITETIMEOUT` | `ui.server.writeTimeout` | `24h` | Max duration for writing a response. Defaults to 24h because server-streaming RPCs (`WatchClients`, `WatchClient`) are hosted on this port and must not be cut off mid-stream. Lower it only if you do not use watch streams. |
| `CLIENT_ETCD_PORT` | `client.etcd.port` | `2379` | Port for the etcd-compatible gRPC API consumed by `etcdctl` and typed clients. |
| `CONFIG_DATA_PATH` | `config.data.path` | `~/.elara/data` (bare binary) / `/var/lib/elara` (container image) | Directory holding the single bbolt state file (`elara.db`). Only one instance may run against it at a time (exclusive file lock). |
| `SERVICE_NAME` | `service.name` | `elara` | Service identity embedded in Prometheus / OTLP resource labels. |
| `SERVICE_VERSION` | `service.version` | _(empty)_ | Service version embedded in Prometheus / OTLP resource labels. |

### Observability

| Env Var | Config Key | Default | Description |
|---|---|---|---|
| `METRICS_ENABLED` | `metrics.enabled` | `false` | When `true`, the HTTP server serves Prometheus-format metrics at `/metrics` (scrapeable by Prometheus / a `ServiceMonitor`). |
| `TRACING_ENABLED` | `tracing.enabled` | `false` | When `true`, Elara creates spans for HTTP requests and gRPC RPCs and pushes them via OTLP. Requires `TRACING_OTLP_ENDPOINT`. |
| `TRACING_OTLP_ENDPOINT` | `tracing.otlp.endpoint` | _(empty)_ | OTLP collector endpoint (OTel Collector, Tempo, or Jaeger OTLP gateway). Required when tracing is enabled; validated at startup. |
| `LOG_LEVEL` | `log.level` | `info` | Structured-log verbosity: `debug` \| `info` \| `warn` \| `error`. |
| `LOG_FORMAT` | `log.format` | `json` | Log output format: `json` \| `text`. |
| `LOG_NOSOURCE` | `log.noSource` | `false` | When `true`, omit source file/line from log records. |

### Authentication

The variables below are listed for completeness. Auth setup — basic-auth vs OIDC,
the bootstrap superadmin flow, and session behavior — is covered in depth on the
[Authentication](authentication.md) page. Do not enable auth from this table
alone; read that page first.

| Env Var | Config Key | Default | Description |
|---|---|---|---|
| `UI_AUTH_ENABLED` | `ui.auth.enabled` | `false` | Master switch for UI/API authentication. When `false`, auth type is forced to `none` and permission checks are skipped. |
| `UI_AUTH_TYPE` | `ui.auth.type` | `none` | `basic-auth` \| `oidc` \| `none`. Anything unrecognized falls back to `none`. |
| `UI_AUTH_BASICAUTH_USERNAME` | `ui.auth.basicAuth.username` | _(empty)_ | Initial superadmin username (required when type is `basic-auth`; **must be email-shaped** — Elara refuses to boot otherwise). |
| `UI_AUTH_BASICAUTH_PASSWORD` | `ui.auth.basicAuth.password` | _(empty)_ | Initial superadmin password (required when type is `basic-auth`). |
| `UI_AUTH_OIDC_ISSUERURL` | `ui.auth.oidc.issuerUrl` | _(empty)_ | OIDC issuer URL. |
| `UI_AUTH_OIDC_CLIENTID` | `ui.auth.oidc.clientId` | _(empty)_ | OIDC client ID. |
| `UI_AUTH_OIDC_CLIENTSECRET` | `ui.auth.oidc.clientSecret` | _(empty)_ | OIDC client secret. |
| `UI_AUTH_OIDC_REDIRECTURL` | `ui.auth.oidc.redirectUrl` | _(empty)_ | OIDC callback/redirect URL. |
| `UI_AUTH_OIDC_SCOPES` | `ui.auth.oidc.scopes` | `openid,email,profile` | OIDC scopes (comma-separated). |
| `UI_AUTH_OIDC_ADMINEMAIL` | `ui.auth.oidc.adminEmail` | _(empty)_ | Email that bootstraps the first superadmin on OIDC (required when type is `oidc`). |
| `UI_AUTH_SESSION_SECURECOOKIE` | `ui.auth.session.secureCookie` | `false` | Marks the session cookie `Secure`. Set to `true` only when served over HTTPS — browsers drop `Secure` cookies on plain HTTP. |

### Client / etcd API

| Env Var | Config Key | Default | Description |
|---|---|---|---|
| `CLIENT_AUTH_ENABLED` | `client.auth.enabled` | `false` | Requires token authentication on the etcd-compatible gRPC API. |
| `CLIENT_HISTORY_MAX_RECORDS` | `client.history.max_records` | `1000` | Max connected-client history records retained. Values `<= 0` fall back to the default. |
| `CLIENT_HISTORY_MAX_AGE` | `client.history.max_age` | `720h` (30 days) | Max age of retained client-history records. Values `<= 0` fall back to the default. |
| `CLIENT_RECENT_EVENTS_CAPACITY` | `client.recent_events.capacity` | `100` | Ring-buffer capacity for recent client events. Values `<= 0` fall back to the default. |

### Demo mode

| Env Var | Config Key | Default | Description |
|---|---|---|---|
| `DEMO_MODE` | `demo.mode` | `false` | Seeds sample namespaces/configs/schemas on startup, injects simulated etcd clients into the monitor, and shows a first-run welcome modal. See the [Quickstart](quickstart.md) for the demo walkthrough. |

### Advanced / dangerous

| Env Var | Config Key | Default | Description |
|---|---|---|---|
| `DANGEROUSLY_SKIP_PERMISSIONS` | `dangerously.skip.permissions` | `false` | **Disables RBAC authorization entirely.** When `true`, every request (UI/ConnectRPC handlers and the etcd token interceptor) bypasses permission enforcement — any authenticated caller gets full access. Never enable this in production; it exists only for local debugging. |

!!! warning "`DANGEROUSLY_SKIP_PERMISSIONS`"
    This flag turns off all authorization checks. It does not weaken permissions
    — it removes them. Leave it unset (`false`) in any shared or production
    environment.

## Docker deployment

The image builds to a `scratch`-based runtime that runs as a non-root user
(UID/GID `65532`), exposes ports `8080` and `2379`, and defaults
`CONFIG_DATA_PATH` to `/var/lib/elara` (declared as a `VOLUME`).

Build the image locally (no published image is required):

```bash
docker build -t elara:latest .
```

For the pre-seeded demo variant, build with `--build-arg DEMO_MODE=true` — see
the [Quickstart](quickstart.md) for the full demo walkthrough.

### Minimal production-ish run

Mount a host directory for the bbolt state so data survives container restarts,
and enable basic-auth:

```bash
docker run -d --name elara \
  -p 8080:8080 \
  -p 2379:2379 \
  -v elara-data:/var/lib/elara \
  -e UI_AUTH_ENABLED=true \
  -e UI_AUTH_TYPE=basic-auth \
  -e UI_AUTH_BASICAUTH_USERNAME=admin@example.com \
  -e UI_AUTH_BASICAUTH_PASSWORD='change-me' \
  -e UI_AUTH_SESSION_SECURECOOKIE=true \
  elara:latest
```

Notes:

- The volume target must match `CONFIG_DATA_PATH` (`/var/lib/elara` in the
  image). A named volume (`elara-data`) or a bind mount both work; only one
  Elara instance may use a given data file at a time.
- Set `UI_AUTH_SESSION_SECURECOOKIE=true` only when the service is reached over
  HTTPS (e.g. behind a TLS-terminating proxy). On plain HTTP the browser drops
  the cookie and login silently fails.
- To scrape metrics, add `-e METRICS_ENABLED=true` and scrape `/metrics` on
  port `8080`.

### docker-compose

```yaml
services:
  elara:
    image: elara:latest
    build: .
    ports:
      - "8080:8080"
      - "2379:2379"
    volumes:
      - elara-data:/var/lib/elara
    environment:
      UI_AUTH_ENABLED: "true"
      UI_AUTH_TYPE: "basic-auth"
      UI_AUTH_BASICAUTH_USERNAME: "admin@example.com"
      UI_AUTH_BASICAUTH_PASSWORD: "change-me"
      UI_AUTH_SESSION_SECURECOOKIE: "true"

volumes:
  elara-data:
```

## Kubernetes deployment

Elara ships an official Helm chart. Every environment variable above maps to a
chart value, and the chart wires up a `Deployment`, `Service`, persistence, and
optional `ServiceMonitor`. The TL;DR:

```bash
helm repo add elara https://sergeyslonimsky.github.io/elara
helm install elara elara/elara
```

For prerequisites, the full `values.yaml` reference, ingress, persistence, and
observability wiring, see the [Helm chart docs](https://github.com/sergeyslonimsky/elara/blob/master/helm/elara/README.md).
