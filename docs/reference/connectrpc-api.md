# ConnectRPC API

Elara's management API is a set of **14 [ConnectRPC](https://connectrpc.com/)
services** served from the HTTP/2 server on port **`8080`**, alongside the
embedded Web UI. The Web UI is itself a client of this API — anything the UI can
do, an external client can do.

## Transport

ConnectRPC handlers speak three protocols on the same endpoint, and the server
picks one per request from the `Content-Type`:

| Protocol | `Content-Type` | Notes |
|----------|----------------|-------|
| **gRPC** | `application/grpc` | Wire-compatible with any gRPC client/codegen |
| **gRPC-Web** | `application/grpc-web` | For browsers / proxies that cannot do trailers |
| **Connect** | `application/json`, `application/proto` | Connect's own protocol — unary calls are plain HTTP POST, so `curl` works |

Procedure URLs follow the standard gRPC path shape,
`/<proto.package>.<Service>/<Method>`, e.g.:

```bash
curl -X POST https://elara.example.com/elara.namespace.v1.NamespaceService/ListNamespaces \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <session-id>' \
  -d '{}'
```

Streaming RPCs (`WatchConfigs`, `WatchClients`, `WatchClient`) are server-streaming
and require HTTP/2 (gRPC / Connect streaming); they are not usable over plain
unary JSON POST.

!!! note "This is not the etcd API"
    Port `8080` is the management/UI surface. Config **consumers** use the
    etcd-compatible gRPC API on port `2379` with a service token — see
    [Client Auth (etcd)](../auth/client-auth.md). The two are separate ports,
    separate protocols, and separate credentials.

## Authentication

Every service except `AuthService` is mounted behind the auth interceptor
(`privateOpts` in `internal/di/service/handler.go`) and requires a **session** —
either the `elara_session` cookie or `Authorization: Bearer <session-id>`
(the Bearer header wins if both are present).

Three services are also **conditionally mounted**: if their feature flag is off,
the routes do not exist at all and calls return HTTP 404, not `Unauthenticated`.

| Service | Auth | Mounted when |
|---------|------|--------------|
| `AuthService` | **Public** — no session required | always |
| `CapabilitiesService` | Session required | always |
| `ProfileService` | Session required | always |
| `ConfigService`, `SchemaService` | Session required | always |
| `NamespaceService` | Session required | always |
| `ClientsService`, `DashboardService`, `FilterService` | Session required | always |
| `TransferService`, `WebhookService` | Session required | always |
| `UserService`, `GroupService` | Session required | `ui.auth.enabled=true` |
| `TokenService` | Session required | `client.auth.enabled=true` |

!!! warning "Forced password change blocks almost everything"
    While the session's user has `password_change_required` set (bootstrap
    basic-auth admin, or after `ResetUserPassword`), the interceptor rejects
    every procedure except exactly three:
    `ProfileService/ChangePassword`, `ProfileService/Me`, and
    `ProfileService/Logout`.

In passthrough mode (`ui.auth.enabled=false`) the interceptor runs with
`skipPermissions=true` and rewrites every request — authenticated or not — as a
synthetic superadmin. Do not expose such an instance.

Beyond authentication, most write RPCs are additionally authorized against the
groups-only RBAC model, with anti-escalation checks (you cannot grant what you
do not hold). `GroupService` and `UserService` document their exact rules in the
`.proto` comments.

---

## AuthService

Login and auth-mode discovery. **The only public service** — it is what an
unauthenticated client calls to obtain a session.

| RPC | Description |
|-----|-------------|
| `GetAuthInfo` | Returns the server's configured `AuthType` (`none` / `basic-auth` / `oidc`) so a client knows which login flow to drive. |
| `OIDCLogin` | Starts the OIDC flow; returns the IdP `redirect_url` to send the browser to. |
| `OIDCCallback` | Exchanges the IdP `code` + `state` for a session; sets the `elara_session` cookie. |
| `BasicLogin` | Password login with `email` + `password`; sets the session cookie and returns `password_change_required`. |

There is no `Login`-side logout — logout lives on `ProfileService`, which is
authenticated by definition.

## ProfileService

The current session's own identity and credentials.

| RPC | Description |
|-----|-------------|
| `Me` | Returns the caller's `email`, `name`, `picture`, `password_change_required`, and the full flattened list of permission assignments the UI uses for client-side gating. |
| `ChangePassword` | Sets a new password. `current_password` is required for a voluntary change but may be omitted when `password_change_required` is set. Basic-auth only — returns `InvalidArgument` under OIDC/passthrough. |
| `Logout` | Revokes the current session and clears the cookie. |

## CapabilitiesService

Server feature flags, so a client can hide UI/skip calls for services that are
not mounted.

| RPC | Description |
|-----|-------------|
| `GetCapabilities` | Returns `etcd_token_auth_enabled` (is `TokenService` mounted?), `user_management_enabled` (are `UserService`/`GroupService` mounted?), and `demo_mode` (instance seeded with sample data). |

---

## ConfigService

CRUD, history, and locking for config entries. This is the primary data-plane
service of the management API.

| RPC | Description |
|-----|-------------|
| `CreateConfig` | Creates a config at `path` in `namespace` with `content`, format, and metadata. |
| `GetConfig` | Reads the current revision of one config by `namespace` + `path`. |
| `UpdateConfig` | Writes new content. Pass `version` for optimistic concurrency. |
| `DeleteConfig` | Deletes a config. |
| `ListConfigs` | Directory-style listing: returns the entries under a folder `path` (`""`/`/` = root), paginated and sortable, with optional name-substring `query`. |
| `GetConfigHistory` | Returns the newest-first revision history for a path (`limit` defaults to 20). |
| `GetConfigAtRevision` | Returns the content of one specific historical revision. |
| `SearchConfigs` | Full search across configs by `query`, optionally scoped to a namespace; paginated. |
| `CopyConfig` | Copies a config from a source `namespace`+`path` to a destination pair (cross-namespace allowed). |
| `ValidateConfig` | Validates candidate `content` against the schema that would apply at `namespace`+`path`, without writing. |
| `WatchConfigs` | **Server stream.** Emits change events for a `path_prefix` within a namespace (`CREATED`, `UPDATED`, `DELETED`, `LOCKED`, `UNLOCKED`, `NAMESPACE_LOCKED`, `NAMESPACE_UNLOCKED`). |
| `GetConfigDiff` | Returns `from_content`, `to_content`, and a rendered textual `diff` between two revisions. |
| `LockConfig` | Marks a single config read-only. |
| `UnlockConfig` | Clears the lock. |

## SchemaService

JSON Schema attachments that validate config content on write.

| RPC | Description |
|-----|-------------|
| `AttachSchema` | Attaches a `json_schema` to a `path_pattern` within a namespace (creates or replaces). |
| `DetachSchema` | Removes the attachment for an exact `path_pattern`. |
| `GetSchema` | Fetches the attachment registered for an exact `path_pattern`. |
| `GetEffectiveSchema` | Resolves which attachment actually applies to a concrete `path` — use this, not `GetSchema`, to answer "what validates this config?". |
| `ListSchemas` | All attachments in a namespace. |

## NamespaceService

Namespaces are the top-level tenancy and RBAC domain unit.

| RPC | Description |
|-----|-------------|
| `CreateNamespace` | Creates a namespace (`name` is validated against the namespace name pattern). |
| `GetNamespace` | Fetches one namespace by name. |
| `UpdateNamespace` | Updates the `description` only — namespace names are immutable. |
| `ListNamespaces` | Paginated, sortable listing with an optional name-substring `query`. |
| `DeleteNamespace` | Deletes a namespace. |
| `LockNamespace` | Marks the whole namespace read-only; emits a `NAMESPACE_LOCKED` watch event. |
| `UnlockNamespace` | Clears the namespace lock. |

---

## UserService

User records and their group memberships. Mounted only when `ui.auth.enabled=true`.

| RPC | Description |
|-----|-------------|
| `ListUsers` | Paginated listing. With `User:Read *` returns everyone; otherwise the result is derived through `Group:Read` — only users in groups the caller can read (unassigned users are invisible). |
| `GetUser` | One user by UUID. `visible_group_ids` is independently filtered by the caller's `Group:Read` scope. |
| `CreateUser` | Creates a user, optionally joining `initial_group_ids` atomically. A password is required in basic-auth mode and must be empty under OIDC. |
| `UpdateUserGroups` | Explicit membership delta (`add_group_ids` / `remove_group_ids`). Adds/removes that are no-ops are accepted; the same id in both lists is `InvalidArgument`. Supports `expected_version` optimistic locking (`FailedPrecondition` on mismatch). |
| `ResetUserPassword` | Admin reset — sets a new password and `password_change_required`. Basic-auth mode only. |
| `DeactivateUser` | Deactivates the user **and revokes all their sessions** in one transaction. Does *not* revoke tokens they issued. |
| `ReactivateUser` | Reactivates a deactivated user. Revoked sessions are not restored — they must log in again. |
| `DeleteUser` | Deletes the user and all their memberships. Basic-auth mode only. System users (`is_system`) cannot be deleted or deactivated. |

## GroupService

Groups are the **only** way permissions reach a user. Mounted only when
`ui.auth.enabled=true`.

| RPC | Description |
|-----|-------------|
| `CreateGroup` | Creates a group, optionally with `initial_permissions`, `initial_member_emails`, and `initial_manager_group_names` (groups granted `Group:Write` over the new group). The caller must hold every permission being granted. |
| `GetGroup` | One group by name. The full permission set is returned; `visible_members` is filtered through the caller's derived `User:Read` scope. |
| `UpdateGroup` | Metadata only — `display_name` and `description`. Members and permissions have their own RPCs by design. |
| `UpdateGroupMembers` | Explicit membership delta by email. Anti-escalation: each added member inherits the group's whole permission set, so the caller must hold all of it. `expected_members_version` gives optimistic locking. |
| `UpdateGroupPermissions` | Explicit permission delta. Checked both per-delta (caller holds each added permission) and as a cascade (caller holds everything the group will hold, if it has members). `expected_permissions_version` gives optimistic locking. |
| `DeleteGroup` | Deletes the group and all of its membership and permission rules in one Casbin write. System groups return `FailedPrecondition`. |
| `ListGroups` | With `Group:Read *` returns every group; otherwise only groups the caller can read individually. An empty page is a valid result, not an error. |

## TokenService

Service credentials for the **etcd-compatible gRPC API** on `:2379`. Mounted only
when `client.auth.enabled=true`.

| RPC | Description |
|-----|-------------|
| `CreateToken` | Issues a token with a `name`, an explicit `namespaces` list (`*` = all), a `permission` (reader/writer — never admin), and an optional `expires_at`. **The `raw_token` is returned exactly once here**; only its hash is stored. |
| `ListTokens` | Paginated, sortable, filterable listing of token metadata (never the secret). |
| `GetToken` | One token's metadata by id. |
| `RevokeToken` | Revokes a token by id. This is the *only* way a token dies early — issuer lifecycle events never cascade to tokens. |

!!! note
    A token's permissions come from its own `Role` and `Namespaces`, not from the
    user who issued it. See [Sessions & Tokens](../auth/sessions-tokens.md#tokens-service-credentials-for-etcd-clients).

---

## ClientsService

Observability over the etcd-API clients currently connected to `:2379`.
Connection history is persisted; per-RPC event logs are in-memory and bounded.

| RPC | Description |
|-----|-------------|
| `ListActiveClients` | All currently-connected clients. |
| `GetClient` | One client (active or recently disconnected) plus its recent in-memory event log. `recent_events` is empty for disconnected clients — those events are not persisted. |
| `ListHistoricalConnections` | Past connections, newest first, `limit`-bounded (0 → server default). |
| `ListClientSessions` | Past connections of the *same logical client*, matched on `client_name` + `k8s_namespace`, newest first; the `current_id` session is excluded if given. |
| `WatchClients` | **Server stream.** Periodic full `SNAPSHOT` updates plus immediate `CONNECTED`/`DISCONNECTED` events. Feeds a live list view; ends when the caller closes the stream. |
| `WatchClient` | **Server stream** for a single client: snapshot updates plus `REQUEST_RECORDED` per-RPC events for a tailing activity log. Ends when that client disconnects. |

## DashboardService

Aggregate counters and a recent-changes feed for the UI home screen.

| RPC | Description |
|-----|-------------|
| `GetStats` | `namespace_count`, `config_count`, `active_client_count`, and the current `global_revision`. |
| `ListActivity` | Recent config change entries (revision, event type, path, namespace, version, timestamp). `limit` defaults to 50. |

## FilterService

Read-only lookup data for building permission-aware pickers and forms. Every
listing RPC accepts an `actions` filter so the UI can ask "which namespaces can
I *write* to?" rather than post-filtering client-side.

| RPC | Description |
|-----|-------------|
| `GetNamespaces` | Namespace picker items, narrowed to those on which the caller holds the requested `actions`. |
| `GetGroups` | Group picker items, same `actions` narrowing. |
| `GetUsers` | User picker items, same `actions` narrowing. |
| `GetPermissionCatalog` | The static catalog for permission-assignment forms: per `PermissionObject`, which `PermissionAction`s are meaningful and what domain the assignment must carry — `GLOBAL` (domain must be `*`), `NAMESPACE` (a namespace name or `*`), or `GROUP` (`group:<id>` or `*`). The server validates assignments against the same catalog. |

## TransferService

Bulk import/export of configs as a bundle (JSON or YAML, optionally zipped).

| RPC | Description |
|-----|-------------|
| `ExportNamespace` | Exports one namespace. Returns raw `data` plus `content_type` and a suggested `filename`. |
| `ExportAll` | Exports every namespace. `zip_layout` selects a single `BUNDLE` file or one file `PER_NAMESPACE`. |
| `ImportNamespace` | Imports a bundle. `on_conflict` is `SKIP` / `OVERWRITE` / `FAIL`; `dry_run` validates without writing; `namespace` overrides the bundle's namespace (and then rejects an all-namespaces bundle). Returns `created` / `updated` / `skipped` / `failed` counts plus per-entry `errors`. |

## WebhookService

Outbound HTTP notifications on config changes.

| RPC | Description |
|-----|-------------|
| `CreateWebhook` | Registers a `url` with a `namespace_filter`, `path_prefix`, an `events` list (`CREATED` / `UPDATED` / `DELETED`), an HMAC `secret`, and an `enabled` flag. |
| `GetWebhook` | One webhook by id. |
| `UpdateWebhook` | Updates any of the above fields by id. |
| `DeleteWebhook` | Removes a webhook. |
| `ListWebhooks` | All registered webhooks. |
| `GetDeliveryHistory` | Recent delivery attempts for one webhook — status, timing, and failures, for debugging endpoints that are not receiving events. |

---

## Generating a client

Protobuf sources live in `proto/elara/<area>/v1/`. `make generate` runs
`buf generate` and emits, per `buf.gen.yaml`:

| Target | Output | Plugin |
|--------|--------|--------|
| Go messages | `internal/proto/elara/<area>/v1/*.pb.go` | `protocolbuffers/go` |
| Go Connect clients/handlers | `internal/proto/elara/<area>/v1/<area>v1connect/*.go` | `connectrpc/go` |
| TypeScript messages | `web/src/gen/elara/<area>/v1/*_pb.ts` | `bufbuild/es` |
| TypeScript TanStack Query hooks | `web/src/gen/elara/<area>/v1/*-<Service>Query.ts` | `connectrpc/query-es` |

Both generated trees are committed, so a Go client can import the
`*v1connect` packages directly. For clients outside this repo, point your own
`buf generate` (or `protoc`) at `proto/` — the service definitions are the
contract, and any gRPC-capable language works via the gRPC protocol.

!!! note "This page is hand-maintained"
    `buf.gen.yaml` has **no** `protoc-gen-doc` plugin — nothing regenerates this
    page. When you add, rename, or remove an RPC, update the relevant table here
    in the same change. The `.proto` files remain the source of truth for exact
    field names, validation rules, and authorization comments.
