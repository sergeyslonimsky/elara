# Core Concepts

This page is a reference for Elara's core building blocks — what each one is,
the fields that matter, and the rules the server actually enforces. It is not a
walkthrough: for hands-on introductions see the
[Quickstart](quickstart.md) and the [To-Do app tutorial](tutorial-todo-app.md).

Elara stores everything in a single [bbolt](https://github.com/etcd-io/bbolt)
file and exposes it three ways: a Web UI, a ConnectRPC API (both on port
`8080`), and an etcd-compatible gRPC API (port `2379`). The concepts below are
shared across all three surfaces.

## Namespaces

A **namespace** is the top-level container for configs. It maps directly to the
first path segment of an etcd key (see [Configs](#configs)), so every config
lives in exactly one namespace.

Fields (`internal/domain/namespace.go`):

- `Name` — canonical identifier, validated as a canonical name (used verbatim
  in etcd keys and RBAC scopes).
- `DisplayName` — optional human-friendly label.
- `Description` — optional free text.
- `Locked` — change-freeze flag (see below).
- `ConfigCount`, `CreatedAt`, `UpdatedAt` — bookkeeping.

Lifecycle operations (`internal/usecase/namespace/`): create, update, delete,
lock, unlock.

**Locking.** Locking a namespace sets `Locked = true`. While a namespace is
locked, every config write inside it — create, update, delete, both via the
Web/ConnectRPC API and the etcd gRPC API — is rejected with `ErrNamespaceLocked`.
Use it as a change freeze to protect a whole environment (e.g. `prod`) from
edits. A locked namespace also cannot be deleted; you must unlock it first.

**Delete guard.** A namespace can only be deleted when it is unlocked *and*
empty. Deleting a namespace that still contains one or more configs fails with a
validation error reporting the config count — Elara never cascades a namespace
delete into its configs.

## Configs

A **config** is a single key/value entry inside a namespace
(`internal/domain/config.go`).

Fields that matter:

- `Path` — must start with `/`, must not contain `//`, and must not end with
  `/` (e.g. `/services/api.yaml`).
- `Content` — the config body, stored normalized.
- `Format` — `json`, `yaml`, or `other`. Parsed via `ParseFormat` (`yml` is
  accepted as an alias for `yaml`); auto-detected from the path extension
  (`.json` / `.yaml` / `.yml`, else `other`) when not set. **Format is
  immutable on update** — you cannot change a config's format after creation.
- `ContentHash` — SHA-256 of the content, used to detect no-op writes.
- `Version` — per-config counter, starts at `1` and increments on each write.
- `Revision` / `CreateRevision` — the [global revision](#global-revision) at
  last modify (etcd `mod_revision`) and at first create.
- `Metadata` — arbitrary string map.
- `Locked` / `NamespaceLocked` — this config's own lock, and whether its
  namespace is locked.

**etcd key mapping.** A config is addressable over the etcd gRPC API as
`/{namespace}{path}`, where the path keeps its leading slash
(`internal/handler/etcdv3/keyencoding.go`):

```
namespace "prod" + path "/services/api.yaml"  →  /prod/services/api.yaml
namespace "default" + path "/foo.json"        →  /default/foo.json
```

The first path segment after the leading `/` is the namespace; everything from
the second `/` onward (leading slash included) is the config path.

**History.** Every write appends a revision-stamped entry to the history
bucket. You can list a config's history and read it as it existed at any past
revision (`internal/usecase/config/service_history.go`, `GetKVAtRevision`) —
this is what backs etcd point-in-time reads and watch-from-revision.

**Config locking.** Individual configs can be locked and unlocked
(`internal/usecase/config/service_update.go`). While a config is locked, any
attempt to update or delete it is rejected with `ErrLocked` — enforced in
storage, so it holds for both the UI and the etcd API (the etcd `Put`/
`DeleteRange` paths return the same error). Lock/unlock transitions are recorded
in a lock-history bucket. Over ConnectRPC a locked write surfaces as a
`failed_precondition` error. Locking a namespace has the same effect on all of
its configs (`NamespaceLocked`).

## Schema Validation

Elara can validate config content against a **JSON Schema** attached to a
namespace (`internal/domain/schema.go`, `internal/usecase/schema/`).

A `SchemaAttachment` binds one JSON Schema to a `(Namespace, PathPattern)` pair.
The pattern is a **glob** (using `github.com/gobwas/glob` with `/` as separator),
so a single schema can cover many configs:

```jsonc
// attach to namespace "prod", pattern "/services/*.json"
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["port"],
  "properties": {
    "port": { "type": "integer", "minimum": 1, "maximum": 65535 }
  }
}
```

**Which schema applies.** When more than one attached pattern matches a config
path, the **most specific pattern wins** — specificity is scored by counting
wildcard characters (`*`, `?`, `[`); fewer wildcards is more specific. On a tie,
the oldest attachment wins (`findBestMatch`). Only the single best match is
applied to a given config.

**When it runs.** Validation happens on every config create and update, before
the write. `other`-format configs are skipped (only `json` and `yaml` are
validated; YAML is re-marshaled through JSON before checking). Schemas are
compiled with `github.com/santhosh-tekuri/jsonschema` v6, which reads the draft
from the schema's `$schema` keyword (through draft 2020-12), and compiled
schemas are cached by content hash.

**On failure.** The write is rejected with a `SchemaValidationError` carrying a
list of violations, each `{path, message, keyword}`. Its message reads:

```
schema validation failed: N violation(s): /field: message [keyword]; ...
```

Over ConnectRPC this maps to an `invalid_argument` error whose message is the
string above, plus a structured `SchemaValidationFailure` error detail
containing every violation's path, message, and keyword
(`internal/handler/v2/errors.go`).

## Bundles

A **bundle** is a portable snapshot of configs used for export/import
(`internal/domain/bundle.go`, `internal/usecase/transfer/`).

- `BundleConfig` — `{Path, Content, Format, Metadata}`.
- `NamespaceBundle` — one namespace and its configs.
- `AllBundle` — every namespace (multi-namespace export/import).

**Export** produces JSON or YAML (encoding chosen per request), optionally
packaged as a **ZIP** for an all-namespaces export (one file per namespace under
`namespaces/<name>.<ext>` plus an index).

**Import** replays a bundle. When a config in the bundle already exists, the
`onConflict` mode decides the outcome (`service_import.go`):

| Mode | Behavior on an existing config |
|------|-------------------------------|
| `SKIP` (default, also used for `UNSPECIFIED`) | Left untouched; counted as *Skipped*. |
| `OVERWRITE` | Existing config is updated (its current `Version` is preserved); counted as *Updated*. |
| `FAIL` | Recorded as an error (`"config already exists"`); counted as *Failed*. |

Configs that don't yet exist are always created. Imports can target an explicit
namespace (overriding the bundle's own namespace field) or infer scope from the
bundle; a multi-namespace `AllBundle` import requires `Transfer:write` on `*`.

**Preview is dry-run, not a diff.** The import preview runs with `dryRun = true`
and returns an `ImportReport` — aggregate counts of `Created` / `Updated` /
`Skipped` / `Failed` plus a per-path error list. It does **not** show a
per-config content diff: an `OVERWRITE` preview reports a config as *Updated*
without telling you what would change. Treat the preview as a summary of how
many entries fall into each bucket, not as a review of the actual changes.

## Clients & Observability

"Clients" in Elara means consumers of the **etcd-compatible gRPC API** — the
services reading configs over the etcd wire protocol. UI and ConnectRPC users
are not clients.

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

The in-memory monitor registry (`internal/service/monitor/registry.go`) tracks,
per connected client: request counts by RPC method, error count, last-activity
time, and the list of **active watches** (each with its key range, start
revision, and flags). It also keeps a small ring buffer of recent per-RPC events
and publishes changes so the UI can update live.

- The **Active** view lists clients that are currently connected
  (`DisconnectedAt == nil`).
- The **History** view shows clients that have since disconnected.

All of this state is in-memory only — it is not persisted and is lost on
restart.

## Webhooks

A **webhook** delivers config-change notifications to an external HTTP endpoint
(`internal/domain/webhook.go`, `internal/usecase/webhook/`).

Fields:

- `URL` — target endpoint (must be `http`/`https`).
- `Events` — any of `created`, `updated`, `deleted` (at least one required).
- `NamespaceFilter` — if set, only events in that namespace fire.
- `PathPrefix` — if set, only configs whose path is at or below that prefix fire
  (boundary-aware: `/svc` matches `/svc` and `/svc/...` but not `/svcold`).
- `Secret` — optional HMAC signing key.
- `Enabled` — disabled webhooks match nothing.

**Delivery & signing.** The dispatcher POSTs a JSON body with
`Content-Type: application/json`. When a `Secret` is set, it computes an
HMAC-SHA256 over the body and sends it in the `X-Elara-Signature` header, formatted
as `sha256=<hex>` (`internal/transport/webhook/dispatcher.go`) — verify it on
your receiver to authenticate the payload.

Each attempt is recorded as a `DeliveryAttempt` (`AttemptNumber`, `StatusCode`,
`LatencyMS`, `Error`, `Success`, `Timestamp`), giving a delivery history with
retries and latency.

## RBAC

Elara uses Casbin RBAC with one hard invariant: **users get permissions only
through group membership** — there is no way to grant a permission directly to a
user. For the rationale and the full model, see
[ADR 0002: Groups-only RBAC](adr/0002-groups-only-rbac.md). This section covers
the practical mechanics (`internal/domain/rbac.go`).

**Permissions are tuples, not canned roles.** A group holds a set of
`Permission{Object, Action, Domain}` grants, assigned explicitly through the API
or UI. There is no fixed "reader gets X, writer gets Y" expansion table on the
web side — you compose the exact objects/actions/scopes a group needs.

- **Objects**: `namespace`, `token`, `client`, `dashboard`, `user`, `group`,
  `policy`, `webhook`, and `*` (all). Note that `namespace` covers *all*
  namespace-scoped content — configs, schemas, and export/import included; there
  is no separate object for those.
- **Actions**: `read`, `write`, `create`, `delete`, and `*` (all). `write`
  implies `read` (you can't edit what you can't read), and `*` matches any
  action.
- **Domain (scope)**: the scope a grant applies to — a namespace
  (`namespace:<name>`), a group (`group:<name>`), or `*` (global / every
  namespace).

**Namespace-scoped vs global.** A grant like `namespace / write / namespace:prod`
lets the group write configs in `prod` only. The same grant with domain `*`
applies to every namespace. Group membership itself is always global.

**The role names.** `admin`, `writer`, and `reader` exist as constants. `admin`
is the wildcard role: the seeded system superadmin group is granted
`* / * / *` (all objects, all actions, all namespaces) as a break-glass policy.
`reader` and `writer` are used primarily as **etcd service-token roles** — a
token carries a role plus a namespace list, where `reader` is read-only and
`writer` additionally permits writes within its namespaces
(`internal/handler/etcdv3/access.go`). (Service tokens are independent
credentials — see the notes on tokens in the project overview.)

## Global Revision

Elara maintains a single **monotonic revision counter** that increments on every
config write across all namespaces — the etcd `mod_revision` model. Each write
stamps the config's `Revision` (and, on create, `CreateRevision`) with the
counter's value and records a history entry at that revision.

This counter is what enables point-in-time reads and etcd
watch-from-revision, and it surfaces on the dashboard as the **Global Revision**
KPI (`internal/usecase/dashboard/service_stats.go`, `CurrentRevision`). Because
it is global, its value reflects total write activity across the whole store,
not any single namespace.
