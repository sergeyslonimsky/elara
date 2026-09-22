# Namespaces & Configs

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
- `Revision` / `CreateRevision` — the
  [global revision](watch-revisions.md#global-revision) at last modify (etcd
  `mod_revision`) and at first create.
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
the second `/` onward (leading slash included) is the config path. See
[Reference → etcd API compatibility](../reference/etcd-compatibility.md) for
the full RPC support matrix.

**History.** Every write appends a revision-stamped entry to the history
bucket. You can list a config's history and read it as it existed at any past
revision (`internal/usecase/config/service_history.go`, `GetKVAtRevision`) —
this is what backs etcd point-in-time reads and watch-from-revision. See
[Watch & Revisions](watch-revisions.md).

**Config locking.** Individual configs can be locked and unlocked
(`internal/usecase/config/service_update.go`). While a config is locked, any
attempt to update or delete it is rejected with `ErrLocked` — enforced in
storage, so it holds for both the UI and the etcd API (the etcd `Put`/
`DeleteRange` paths return the same error). Lock/unlock transitions are recorded
in a lock-history bucket. Over ConnectRPC a locked write surfaces as a
`failed_precondition` error. Locking a namespace has the same effect on all of
its configs (`NamespaceLocked`).
