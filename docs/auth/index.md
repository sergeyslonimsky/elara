# Auth & Access

This section covers how to turn on authentication, seed the first admin, and
understand the session/token/permission model well enough to deploy Elara from
scratch.

Elara has **two independent auth surfaces**:

| Surface | Who uses it | Credential | Config toggle |
|---------|-------------|------------|---------------|
| **UI / ConnectRPC API** (`:8080`) | Operators via the Web UI, and CLI/API clients | A **session** (cookie or Bearer) | `ui.auth.enabled` |
| **etcd-compatible gRPC API** (`:2379`) | 3rd-party services reading/writing config | A **service token** | `client.auth.enabled` |

These are configured separately. It is normal to enable UI auth while keeping
the etcd port token-protected (production), or to disable both for a local demo.

- [Basic Auth](basic.md) — a local username/password admin, no external IdP.
- [OIDC](oidc.md) — delegate login to an existing identity provider.
- [Passthrough](passthrough.md) — no authentication at all (dev/demo only).
- [Sessions & Tokens](sessions-tokens.md) — the two credential types and how
  they differ.
- [Client Auth (etcd)](client-auth.md) — turning token enforcement on/off for
  the etcd-compatible gRPC port.
- [RBAC, Groups & Roles](rbac-groups.md) — the permission model and how to
  grant access.

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

## Configuration keys

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
| `client.auth.enabled` | `CLIENT_AUTH_ENABLED` | Token auth for the etcd gRPC port — see [Client Auth](client-auth.md) |

!!! warning "`secureCookie` and plain HTTP"
    `ui.auth.session.secureCookie` marks the `elara_session` cookie with the
    `Secure` flag. Set it to **`false`** for plain-HTTP local/dev setups —
    browsers drop `Secure` cookies over HTTP (RFC 6265), so with it `true` on
    HTTP you will never stay logged in. Set it to **`true`** in production
    behind TLS.

## Boot-time validation

Config is validated at startup (`UIAuthConfig.Validate`); the service **refuses
to boot** on a misconfiguration:

- `basic-auth` without a username → `basic-auth requires ui.auth.basicAuth.username to be set`
- `basic-auth` without a password → `basic-auth requires ui.auth.basicAuth.password to be set`
- `oidc` without `adminEmail` → `oidc requires ui.auth.oidc.adminEmail to be set`
