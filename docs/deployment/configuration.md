# Configuration reference

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

## Server & data

| Env Var | Config Key | Default | Description |
|---|---|---|---|
| `UI_SERVER_PORT` | `ui.server.port` | `8080` | Port for the HTTP/2 server that serves the Web UI, the ConnectRPC API, and (when enabled) `/metrics`. |
| `UI_SERVER_READTIMEOUT` | `ui.server.readTimeout` | `0` (no timeout) | Max duration for reading a request. Accepts Go durations (`5s`, `250ms`). |
| `UI_SERVER_WRITETIMEOUT` | `ui.server.writeTimeout` | `24h` | Max duration for writing a response. Defaults to 24h because server-streaming RPCs (`WatchClients`, `WatchClient`) are hosted on this port and must not be cut off mid-stream. Lower it only if you do not use watch streams. |
| `CLIENT_ETCD_PORT` | `client.etcd.port` | `2379` | Port for the etcd-compatible gRPC API consumed by `etcdctl` and typed clients. |
| `CONFIG_DATA_PATH` | `config.data.path` | `~/.elara/data` (bare binary) / `/var/lib/elara` (container image) | Directory holding the single bbolt state file (`elara.db`). Only one instance may run against it at a time (exclusive file lock). |
| `SERVICE_NAME` | `service.name` | `elara` | Service identity embedded in Prometheus / OTLP resource labels. |
| `SERVICE_VERSION` | `service.version` | _(empty — falls back to the build-stamped version, see `elara version`)_ | Service version embedded in Prometheus / OTLP resource labels. |

## Observability

| Env Var | Config Key | Default | Description |
|---|---|---|---|
| `METRICS_ENABLED` | `metrics.enabled` | `false` | When `true`, the HTTP server serves Prometheus-format metrics at `/metrics` (scrapeable by Prometheus / a `ServiceMonitor`). |
| `TRACING_ENABLED` | `tracing.enabled` | `false` | When `true`, Elara creates spans for HTTP requests and gRPC RPCs and pushes them via OTLP. Requires `TRACING_OTLP_ENDPOINT`. |
| `TRACING_OTLP_ENDPOINT` | `tracing.otlp.endpoint` | _(empty)_ | OTLP collector endpoint (OTel Collector, Tempo, or Jaeger OTLP gateway). Required when tracing is enabled; validated at startup. |
| `LOG_LEVEL` | `log.level` | `info` | Structured-log verbosity: `debug` \| `info` \| `warn` \| `error`. |
| `LOG_FORMAT` | `log.format` | `json` | Log output format: `json` \| `text`. |
| `LOG_NOSOURCE` | `log.noSource` | `false` | When `true`, omit source file/line from log records. |

See [Concepts → Clients & Observability](../concepts/clients-observability.md)
for what the connected-clients monitor tracks separately from these metrics.

## Authentication

The variables below are listed for completeness. Auth setup — basic-auth vs OIDC,
the bootstrap superadmin flow, and session behavior — is covered in depth in
[Auth & Access](../auth/index.md). Do not enable auth from this table
alone; read that section first.

| Env Var | Config Key | Default | Description |
|---|---|---|---|
| `UI_AUTH_ENABLED` | `ui.auth.enabled` | `false` | Master switch for UI/API authentication. When `false`, auth type is forced to `none` and permission checks are skipped. |
| `UI_AUTH_TYPE` | `ui.auth.type` | `none` | `basic-auth` \| `oidc` \| `none`. Empty is treated as `none` (Helm's chart always sets this variable, so an empty value is expected, not a mistake). Any other unrecognized value fails startup instead of silently falling back — fix the typo rather than relying on a fallback. |
| `UI_AUTH_BASICAUTH_USERNAME` | `ui.auth.basicAuth.username` | _(empty)_ | Initial superadmin username (required when type is `basic-auth`; **must be email-shaped** — Elara refuses to boot otherwise). |
| `UI_AUTH_BASICAUTH_PASSWORD` | `ui.auth.basicAuth.password` | _(empty)_ | Initial superadmin password (required when type is `basic-auth`). |
| `UI_AUTH_OIDC_ISSUERURL` | `ui.auth.oidc.issuerUrl` | _(empty)_ | OIDC issuer URL. |
| `UI_AUTH_OIDC_CLIENTID` | `ui.auth.oidc.clientId` | _(empty)_ | OIDC client ID. |
| `UI_AUTH_OIDC_CLIENTSECRET` | `ui.auth.oidc.clientSecret` | _(empty)_ | OIDC client secret. |
| `UI_AUTH_OIDC_REDIRECTURL` | `ui.auth.oidc.redirectUrl` | _(empty)_ | OIDC callback/redirect URL. |
| `UI_AUTH_OIDC_SCOPES` | `ui.auth.oidc.scopes` | `openid,email,profile` | OIDC scopes (comma-separated). |
| `UI_AUTH_OIDC_ADMINEMAIL` | `ui.auth.oidc.adminEmail` | _(empty)_ | Email that bootstraps the first superadmin on OIDC (required when type is `oidc`). |
| `UI_AUTH_SESSION_SECURECOOKIE` | `ui.auth.session.secureCookie` | `false` | Marks the session cookie `Secure`. Set to `true` only when served over HTTPS — browsers drop `Secure` cookies on plain HTTP. |

## Client / etcd API

| Env Var | Config Key | Default | Description |
|---|---|---|---|
| `CLIENT_AUTH_ENABLED` | `client.auth.enabled` | `false` | Requires token authentication on the etcd-compatible gRPC API. See [Client Auth (etcd)](../auth/client-auth.md). |
| `CLIENT_HISTORY_MAX_RECORDS` | `client.history.max_records` | `1000` | Max connected-client history records retained. Values `<= 0` fall back to the default. |
| `CLIENT_HISTORY_MAX_AGE` | `client.history.max_age` | `720h` (30 days) | Max age of retained client-history records. Values `<= 0` fall back to the default. |
| `CLIENT_RECENT_EVENTS_CAPACITY` | `client.recent_events.capacity` | `100` | Ring-buffer capacity for recent client events. Values `<= 0` fall back to the default. |

## Demo mode

| Env Var | Config Key | Default | Description |
|---|---|---|---|
| `DEMO_MODE` | `demo.mode` | `false` | Seeds sample namespaces/configs/schemas on startup, injects simulated etcd clients into the monitor, and shows a first-run welcome modal. See the [Quickstart](../getting-started/quickstart.md) for the demo walkthrough. |

## Advanced / dangerous

| Env Var | Config Key | Default | Description |
|---|---|---|---|
| `DANGEROUSLY_SKIP_PERMISSIONS` | `dangerously.skip.permissions` | `false` | **Disables RBAC authorization entirely.** When `true`, every request (UI/ConnectRPC handlers and the etcd token interceptor) bypasses permission enforcement — any authenticated caller gets full access. Never enable this in production; it exists only for local debugging. |

!!! warning "`DANGEROUSLY_SKIP_PERMISSIONS`"
    This flag turns off all authorization checks. It does not weaken permissions
    — it removes them. Leave it unset (`false`) in any shared or production
    environment.

    It cannot be combined with `UI_AUTH_ENABLED=true` or `CLIENT_AUTH_ENABLED=true`
    — Elara refuses to start rather than silently running with a login screen
    (or a Tokens UI) that looks like it enforces access when it doesn't. Use it
    only for a fully open instance with both of those left `false`.
