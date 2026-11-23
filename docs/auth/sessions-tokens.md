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

- `Name` — the **prefix** the issuer picks: lowercase letters, digits, `.`,
  `_` and `-`, starting and ending with a letter or digit, at most 48
  characters. Names may repeat across tokens.
- `Username` — `Name` + `-` + a 6-character random suffix the server
  generates, e.g. `billing-ci-k7x2pq`. Globally unique and immutable; it is the
  login an etcd client presents (see below).
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
You choose the name prefix; the server appends the random suffix. The response
carries the **username and the raw token, and the raw token is returned exactly
once** (only its SHA-256 hash is stored) — copy both then; the token cannot be
retrieved again. Raw tokens are prefixed `elara_`.

!!! note "Why the suffix"
    A username has to be unique across all tokens, but users only see the
    tokens in their own namespaces. If the name alone were the login, creating
    `ci` would fail with "already exists" because of a token the user cannot
    see — revealing that it exists. A server-generated suffix makes collisions
    a non-event, so nothing about invisible tokens leaks.

Tokens created before usernames existed get one on the next start, derived
from their old name (normalized to the allowed characters, falling back to
`token` when nothing usable remains). The assigned username is shown in the
token list.

### Using a token against the etcd gRPC API

A token is a **user name + password** pair as far as etcd clients are
concerned: the username is the user name, the raw token is the password. This
is etcd's own login flow, so stock tools work unchanged.

=== "etcdctl"

    ```bash
    etcdctl --endpoints=elara.example.com:2379 \
      --user 'billing-ci-k7x2pq:elara_<rest-of-token>' \
      get /billing/app.yaml
    ```

=== "Go (clientv3)"

    ```go
    cli, err := clientv3.New(clientv3.Config{
        Endpoints: []string{"elara.example.com:2379"},
        Username:  "billing-ci-k7x2pq",
        Password:  os.Getenv("ELARA_TOKEN"), // elara_...
    })
    ```

=== "Raw gRPC metadata"

    ```text
    token: elara_<rest-of-token>
    ```

    or, equivalently, the original form:

    ```text
    authorization: Bearer elara_<rest-of-token>
    ```

    `clientv3.Config{Token: "elara_..."}` sends the first form without a login
    round-trip.

How it works on the wire: the client calls `Auth.Authenticate(name,
password)`; Elara checks the pair and returns the raw token as the etcd auth
token, which the client then attaches to every call as `token` metadata.
Elara keeps no login state — the token is re-validated on every request, so
revoking it takes effect immediately. A wrong user name, wrong password and
expired token all fail identically with
`etcdserver: authentication failed, invalid user ID or password`.

The gRPC token interceptor (`internal/handler/etcdv3/interceptor/auth.go`)
hashes the presented token, looks it up, rejects unknown or expired tokens with
`etcdserver: invalid auth token` (`Unauthenticated`), records last-used
IP/time, and injects the token's namespace/role claims. Every KV and Watch RPC
is then checked against those claims: reads need the namespace in scope;
writes additionally need `writer` role; scan-all watches need a `*` (wildcard)
token.

!!! warning "Use TLS"
    The password is the token itself. Over a plaintext connection it is as
    exposed as the `authorization` header always was — terminate TLS in front
    of `:2379` anywhere the network isn't trusted.

This requires `client.auth.enabled=true` — see [Client Auth](client-auth.md).
