# Authentication & Authorization

This page covers how to turn on authentication, seed the first admin, and
understand the session/token model well enough to deploy Elara from scratch.

Elara has **two independent auth surfaces**:

| Surface | Who uses it | Credential | Config toggle |
|---------|-------------|------------|---------------|
| **UI / ConnectRPC API** (`:8080`) | Operators via the Web UI, and CLI/API clients | A **session** (cookie or Bearer) | `ui.auth.enabled` |
| **etcd-compatible gRPC API** (`:2379`) | 3rd-party services reading/writing config | A **service token** | `client.auth.enabled` |

These are configured separately. It is normal to enable UI auth while keeping
the etcd port token-protected (production), or to disable both for a local demo.

---

## Auth Types Overview

UI authentication has three types, selected by `ui.auth.type` when
`ui.auth.enabled=true`. When `ui.auth.enabled=false`, the type is forced to
`none` regardless of what `ui.auth.type` says.

| Type | `ui.auth.type` | First admin comes from | Use when |
|------|----------------|------------------------|----------|
| **None (passthrough)** | `none` (or auth disabled) | A synthetic local admin injected on every request | Local dev / demo only — **no login, no protection** |
| **Basic Auth** | `basic-auth` | A local user seeded from configured username/password | Single-operator setups, air-gapped installs, quick production stand-up |
| **OIDC** | `oidc` | The first login whose verified email matches `adminEmail` | Real deployments with an existing IdP (Okta, Keycloak, Auth0, Google, …) |

The auth type is a domain constant (`internal/domain/user.go`):
`AuthTypeNone = "none"`, `AuthTypeBasicAuth = "basic-auth"`, `AuthTypeOIDC = "oidc"`.

### Configuration keys

Every key below is Viper-backed; the environment variable form (uppercased,
dot → underscore) overrides file config.

| Config key | Env var | Notes |
|------------|---------|-------|
| `ui.auth.enabled` | `UI_AUTH_ENABLED` | Master switch for UI auth |
| `ui.auth.type` | `UI_AUTH_TYPE` | `none` / `basic-auth` / `oidc` |
| `ui.auth.basicAuth.username` | `UI_AUTH_BASICAUTH_USERNAME` | Required for basic-auth (must be email-shaped) |
| `ui.auth.basicAuth.password` | `UI_AUTH_BASICAUTH_PASSWORD` | Required for basic-auth |
| `ui.auth.oidc.issuerUrl` | `UI_AUTH_OIDC_ISSUERURL` | OIDC discovery issuer |
| `ui.auth.oidc.clientId` | `UI_AUTH_OIDC_CLIENTID` | |
| `ui.auth.oidc.clientSecret` | `UI_AUTH_OIDC_CLIENTSECRET` | |
| `ui.auth.oidc.redirectUrl` | `UI_AUTH_OIDC_REDIRECTURL` | Must match the IdP-registered callback |
| `ui.auth.oidc.scopes` | `UI_AUTH_OIDC_SCOPES` | Defaults to `openid email profile` |
| `ui.auth.oidc.adminEmail` | `UI_AUTH_OIDC_ADMINEMAIL` | Required for oidc — bootstraps the first admin |
| `ui.auth.session.secureCookie` | `UI_AUTH_SESSION_SECURECOOKIE` | See below |
| `client.auth.enabled` | `CLIENT_AUTH_ENABLED` | Token auth for the etcd gRPC port |

!!! warning "`secureCookie` and plain HTTP"
    `ui.auth.session.secureCookie` marks the `elara_session` cookie with the
    `Secure` flag. Set it to **`false`** for plain-HTTP local/dev setups —
    browsers drop `Secure` cookies over HTTP (RFC 6265), so with it `true` on
    HTTP you will never stay logged in. Set it to **`true`** in production
    behind TLS.

### Boot-time validation

Config is validated at startup (`UIAuthConfig.Validate`); the service **refuses
to boot** on a misconfiguration:

- `basic-auth` without a username → `basic-auth requires ui.auth.basicAuth.username to be set`
- `basic-auth` without a password → `basic-auth requires ui.auth.basicAuth.password to be set`
- `oidc` without `adminEmail` → `oidc requires ui.auth.oidc.adminEmail to be set`

---

## Basic Auth Setup

On startup with `basic-auth`, Elara seeds a single local admin user from the
configured credentials, adds it to the system superadmin group, and marks it as
a protected system user (`User.System = true`, so it can never be deactivated or
deleted through the API). The username **must be email-shaped** — it is
normalized into the user's login identity and fails fast at startup otherwise.

### Environment

```bash
UI_AUTH_ENABLED=true
UI_AUTH_TYPE=basic-auth
UI_AUTH_BASICAUTH_USERNAME=admin@example.com
UI_AUTH_BASICAUTH_PASSWORD=change-me-now
# Plain HTTP locally? keep this false. Behind TLS in prod? set true.
UI_AUTH_SESSION_SECURECOOKIE=false
```

### Required first step: forced password change

The bootstrap admin is created with `PasswordChangeRequired = true`. This is
**enforced server-side at the auth interceptor**, not merely a UI nag:

1. Log in with the bootstrap credentials. `BasicLogin` succeeds and the
   response carries `passwordChangeRequired: true`.
2. Until the password is changed, **every** other API call is rejected with
   `PermissionDenied` / "password change required". Only three procedures are
   permitted in this state:
   `ProfileService/ChangePassword`, `ProfileService/Me`, and
   `ProfileService/Logout`.
3. Call `ProfileService.ChangePassword` with a `new_password`. When the change
   is forced, `current_password` may be left empty; supply it for a normal
   voluntary change later. A successful change clears the flag and issues a
   fresh session cookie.

!!! note
    `ProfileService.ChangePassword` is only available when the auth type is
    `basic-auth`. Under OIDC or passthrough it returns
    `InvalidArgument` / "feature not available" — passwords are the IdP's job
    (OIDC) or nonexistent (passthrough).

After the password change you have a normal authenticated session and full
superadmin rights.

---

## OIDC Setup

Under OIDC, Elara does **not** self-provision arbitrary users
(no JIT provisioning): an unknown identity is rejected with
`ErrIdentityNotProvisioned`. The one exception is the configured admin, which is
bootstrapped specially.

### Environment

```bash
UI_AUTH_ENABLED=true
UI_AUTH_TYPE=oidc
UI_AUTH_OIDC_ISSUERURL=https://idp.example.com/
UI_AUTH_OIDC_CLIENTID=elara
UI_AUTH_OIDC_CLIENTSECRET=...
UI_AUTH_OIDC_REDIRECTURL=https://elara.example.com/api/auth/callback
UI_AUTH_OIDC_ADMINEMAIL=admin@example.com
UI_AUTH_SESSION_SECURECOOKIE=true
```

### How the admin bootstrap actually works

This is a **one-time link at bootstrap + first login**, not a per-login check:

1. **At startup**, `BootstrapOIDC` creates a placeholder system user whose
   `Email` equals `adminEmail` and whose identity list is intentionally empty,
   then adds that placeholder to the superadmin group. The superadmin group and
   its wildcard policy are created here too.
2. **On the first OIDC callback** whose token carries a *verified* email
   (`email_verified=true`) equal to `adminEmail`, Elara's email-fallback linking
   attaches the real `(oidc, subject)` identity to that placeholder user. From
   that point the user is a fully-linked superadmin.
3. **Subsequent logins** resolve directly by `(provider, subject)` — the admin
   email is **not** re-checked on every login. The membership rule is written
   once at bootstrap and is a protected system invariant; the runtime
   re-promotion that older designs had was removed.

An anti-hijack guard applies: if the target user already has an identity for the
same provider, a second IdP user presenting the same email cannot steal the
account (they are rejected with `ErrIdentityNotProvisioned`).

Non-admin users must be provisioned in Elara first (created as users with
matching email), then linked on their first verified OIDC login the same way.

---

## Passthrough / No-Auth Mode

!!! danger "Development only"
    Passthrough mode performs **no authentication at all**. Every request to the
    UI/ConnectRPC API is treated as a superadmin. Never expose a passthrough
    instance on a network anyone else can reach.

Enable it by simply leaving UI auth off:

```bash
UI_AUTH_ENABLED=false
# ui.auth.type is ignored and forced to "none"
```

How it works: at startup `BootstrapPassthrough` seeds a synthetic system user
(`local-admin@elara.internal`, `User.System = true`) into the superadmin group.
The auth interceptor runs in a bypass mode (`skipPermissions=true`) — any
request, authenticated or not, is rewritten with this synthetic admin context,
so enforcement uniformly resolves to superadmin. There is no login screen and no
session to obtain. A one-shot `WARN` is logged at startup so the bypass is
grep-able in production-shaped logs.

Because there is no real principal, the authorization PDP must also run with
permissions skipped — the two are wired in lockstep from the same
`ui.auth.enabled` flag, so you cannot accidentally get a half-open state through
config alone.

---

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
request even if they hold a still-valid session id.

---

## Tokens (service credentials for etcd clients)

A **token** is a service credential for 3rd-party clients hitting the
etcd-compatible gRPC API (`:2379`). Tokens are **not** sessions and **not**
personal access tokens:

| | Session | Token |
|---|---------|-------|
| Purpose | User auth on the UI/ConnectRPC API | Service auth on the etcd gRPC API |
| Permissions | The user's group-derived RBAC | The token's **own** `Role` + `Namespaces` |
| Lifecycle | Tied to the user (deactivation cascades) | **Independent of the issuing user** |
| Storage | `sessions` bucket | Token store, hashed (SHA-256) |

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

```
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

---

## Enabling / Disabling etcd Client Auth

Token enforcement on the etcd gRPC port is gated by `client.auth.enabled`
(env `CLIENT_AUTH_ENABLED`):

```bash
CLIENT_AUTH_ENABLED=true   # production: every etcd RPC needs a valid token
```

!!! danger "Never disable in production"
    With `CLIENT_AUTH_ENABLED=false`, the etcd-compatible API requires **no
    token at all** — anyone who can reach port `2379` can read and write **every
    config in every namespace**. This flag exists for local dev/demo only. Leave
    it **`true`** anywhere the port is reachable by anything you don't fully
    trust.

When disabled, the interceptor injects wildcard admin-equivalent claims for
every call (nil claims are treated as "auth disabled → always allowed" in the
per-namespace checks), which is why unauthenticated clients get full access.

---

## Groups & Roles (making someone an admin)

Elara uses **groups-only RBAC**: users receive permissions **only** through
group membership. There is no API to grant a role directly to a user. The
rationale is documented in
[ADR 0002 — Groups-only RBAC](adr/0002-groups-only-rbac.md); this section is the
practical how-to.

### Roles and what they grant

Roles (`internal/domain/rbac.go`) describe the object/action level a grant
covers. Permissions are stored as `(object, action, domain)` tuples, where
`domain` is a namespace (`namespace:<name>`) or the wildcard `*`:

| Role | Object / Action semantics |
|------|---------------------------|
| `admin` | All actions (`*`) on all objects (`*`) — full control |
| `writer` | `write` on content (write **implies** read) |
| `reader` | `read` on content |

Action semantics (`ActionGrants`): a `write` grant also satisfies `read`
(you cannot edit what you cannot read); `create` and `delete` are independent
capabilities. Objects include `namespace`, `token`, `user`, `group`, `policy`,
`webhook`, `dashboard`, plus the `*` wildcard.

The system superadmin group carries the single break-glass rule
`(group:superadmin, *, *, *)` — all objects, all actions, all namespaces — and
its members are global admins.

### Practical flow: grant someone admin over a namespace (or globally)

1. **Create a group** (Web UI *Groups* → *Create*, or `GroupService.Create`).
   You can attach initial permissions and members atomically at creation time.
2. **Assign the group a permission** in a domain: pick the object/action and set
   the domain to a specific namespace (`namespace:prod`) or `*` for all
   namespaces. For a namespace admin, grant all-actions-on-all-objects scoped to
   that namespace; for a platform admin, scope it to `*` (or just add them to
   the built-in superadmin group).
3. **Add the user to the group.** Membership is what actually confers the
   permissions.

Anti-escalation is enforced throughout: you cannot grant a group (or its
members) any permission you do not yourself already hold. The bootstrap admin
(basic-auth local user, or the OIDC `adminEmail` identity) starts in the
superadmin group and can therefore set up all other groups.
