# Sessions & Tokens

Elara has two distinct credential types. Mixing them up is the most common
source of confusion when integrating a client:

| | Session | Token |
|---|---------|-------|
| Purpose | User auth on the UI/ConnectRPC API | Service auth on the etcd gRPC API |
| Permissions | The user's group-derived RBAC | The token's **own** `Role` + `Namespaces` |
| Lifecycle | Tied to the user (deactivation cascades) | **Independent of the issuing user** |
| Storage | `sessions` bucket | Token store, hashed (SHA-256) |

## Sessions

A **session** is user authentication state for the UI/ConnectRPC API. It is
server-side and opaque: the id is a 256-bit cryptographically random value,
base64-URL-encoded — no JWT, no embedded claims. Sessions live in the bbolt
`sessions` bucket.

### Two transports, one entity

| Client | How the session id travels |
|--------|----------------------------|
| Web UI | `elara_session` cookie (HttpOnly, `SameSite=Lax`, `Path=/`, `Secure` per `secureCookie`) |
| CLI / API clients | `Authorization: Bearer <session-id>` header |

If **both** are present on one request, the **Bearer header wins** (it is checked
before the cookie).

### TTLs

Verified against `internal/service/auth/sessions/model.go`:

| Session type | Initial TTL | Sliding? | Hard cap |
|--------------|-------------|----------|----------|
| Web (`ClientTypeWeb`) | 8 hours | Yes — extended on activity | 30 days from creation |
| CLI (`ClientTypeCLI`) | 30 days | No (absolute in MVP) | 30 days |

Web sliding-TTL refresh is throttled (~60s) and only extends when the delta
exceeds ~5 minutes, so an active session keeps rolling forward its 8-hour window
until it hits the 30-day hard cap from creation.

### Audit log

Every **active** state change appends a `SessionEvent` to the `session_events`
bucket (append-only audit trail). Event types:

- `created` — a session was minted (login)
- `refreshed` — a sliding-TTL extension was applied (web only)
- `revoked_by_user` — user logged out / revoked their own session
- `revoked_by_admin` — an admin revoked it
- `revoked_cascade` — revoked as a side effect (e.g. user deactivation)

**Passive expiration is not logged.** When `ExpiresAt < now` the session simply
stops authenticating; the expiration moment is observable directly from the
row's `ExpiresAt`. The audit log records observed actions, not time-driven
transitions.

### Enforcement on every request

The auth interceptor, per request: extract session id (Bearer beats cookie) →
`SessionService.Validate` (rejects not-found / revoked / expired) → load the user
→ reject if `User.Status != active` → best-effort throttled `Refresh` → inject
session + user into context. A deactivated user cannot make any authenticated
request even if they hold a still-valid session id — see
[Users & Groups lifecycle](users-lifecycle.md).

## Tokens (service credentials for etcd clients)

A **token** is a service credential for 3rd-party clients hitting the
etcd-compatible gRPC API (`:2379`). Tokens are **not** sessions and **not**
personal access tokens.

A token's permissions come **only** from its own fields, never inherited from the
issuer. Deactivating, renaming, or deleting the user who issued a token has **no
effect** on that token — tokens must be revoked explicitly via
`TokenService.Revoke`.

### Token fields

Defined in `internal/domain/token.go`:

- `Role` — either `writer` or `reader` **only** (a token can never be `admin`).
  - `reader` → read allowed.
  - `writer` → read + write allowed (write implies read).
- `Namespaces` — explicit non-empty list of namespaces the token may touch;
  `*` grants all namespaces (required for unbounded / scan-all watches).
- `ExpiresAt` — optional; `nil` means the token never expires.

A token can never grant more than its creator holds: issuing a `writer` token
for namespace `prod` requires the creator to have write on `prod`, otherwise the
request fails with a permission-escalation error.

### Issuing a token

Via the Web UI (Tokens page) or `TokenService.Create` on the ConnectRPC API.
The **raw token string is returned exactly once at creation** (only its SHA-256
hash is stored) — copy it then; it cannot be retrieved again. Raw tokens are
prefixed `elara_`.

### Using a token against the etcd gRPC API

Send it as gRPC metadata on the `:2379` connection:

```text
authorization: Bearer elara_<rest-of-token>
```

The gRPC token interceptor (`internal/handler/etcdv3/interceptor/auth.go`)
hashes the presented token, looks it up, rejects unknown (`invalid token`) or
expired (`token expired`) tokens, records last-used IP/time, and injects the
token's namespace/role claims. Every KV and Watch RPC is then checked against
those claims: reads need the namespace in scope; writes additionally need
`writer` role; scan-all watches need a `*` (wildcard) token.

With an etcd client library, set the metadata credential per your library's API
(e.g. a per-RPC credential that emits the `authorization` header). Example with
`etcdctl`-style usage is client-specific; the wire requirement is simply the
`authorization: Bearer elara_…` metadata pair above.

This requires `client.auth.enabled=true` — see [Client Auth](client-auth.md).
