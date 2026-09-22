# Errors catalog

All of Elara's domain-level sentinel errors (`internal/domain/errors.go`,
23 values) and how they map to ConnectRPC status codes. Use `errors.Is` against
these sentinels in Go client code; ConnectRPC/HTTP clients see the codes below.

## Mapping

Most domain errors are mapped centrally in `internal/handler/v2/errors.go`
(`ToConnectError`); a few auth-specific ones are mapped closer to where they're
produced (noted below).

| Domain error | Connect code | Meaning |
|---|---|---|
| `ErrNotFound` | `NotFound` | Resource doesn't exist. |
| `ErrAlreadyExists` | `AlreadyExists` | Resource with that identity already exists. |
| `ErrIdentityNotProvisioned` | `PermissionDenied` | OIDC login for an identity with no pre-provisioned user record (no JIT provisioning). |
| `ErrIdentityTaken` | `AlreadyExists` | Another user already has this `(provider, subject)` identity. |
| `ErrVersionConflict` | `Aborted` | Optimistic-concurrency mismatch — the resource changed since you read it; retry with the current version. |
| `ErrLocked` / `ErrNamespaceLocked` | `FailedPrecondition` | Write blocked by a config-level or namespace-level lock. See [Namespaces & Configs → Locking](../concepts/namespaces-configs.md#namespaces). |
| `ErrInvalidFormat` | `InvalidArgument` | Config format is neither `json`, `yaml`, nor auto-detectable. |
| `ErrUnauthorized` | `Unauthenticated` | No valid credential presented. |
| `ErrForbidden` | `PermissionDenied` | Credential valid, but RBAC denies the action. |
| `ErrPermissionEscalation` | `PermissionDenied` | You tried to grant a permission you don't hold yourself. See [RBAC, Groups & Roles](../auth/rbac-groups.md). |
| `ErrSystemImmutable` | `PermissionDenied` | Target is a system-protected entity (`User.System`/`Group.System`) — see [Users & Groups lifecycle](../auth/users-lifecycle.md#system-protected-entities). |
| `ErrInvalidToken` | `Unauthenticated` | Service token not found (etcd gRPC API). |
| `ErrUserDeactivated` | `PermissionDenied` | The authenticated user's account is deactivated. See [Users & Groups lifecycle](../auth/users-lifecycle.md). |
| `SchemaValidationError` (not a sentinel — a typed error) | `InvalidArgument` | JSON Schema violation; response carries a structured `SchemaValidationFailure` detail with per-field violations. See [Schema Validation](../concepts/schema-validation.md). |
| `ValidationError` (typed) | `InvalidArgument` | Generic field validation failure. |
| *(anything else)* | `Internal` | Unmapped/unexpected error — treat as a bug report. |

## Auth-specific mappings (not in the central switch)

These are mapped where they're produced, not in `ToConnectError`:

| Domain error | Connect code | Where |
|---|---|---|
| `ErrSessionNotFound` | `Unauthenticated` | `internal/handler/v2/interceptor/auth.go` — session id doesn't resolve. |
| `ErrSessionExpired` | `Unauthenticated` | Same interceptor — session TTL passed. |
| `ErrSessionRevoked` | `Unauthenticated` | Same interceptor — session was explicitly revoked. |
| `ErrPasswordChangeRequired` | `PermissionDenied` | Same interceptor — blocks every RPC except the forced-change allowlist. See [Basic Auth Setup](../auth/basic.md#required-first-step-forced-password-change). |
| `ErrFeatureNotAvailable` | `InvalidArgument` | Handler-level (auth/user/profile handlers) — an operation that doesn't apply to the current auth type, e.g. `ChangePassword` under OIDC, or user management when `client.auth`/`ui.auth` capabilities are off. |

## Not centrally mapped (fall through to `Internal`)

`ErrInvalidIdentityProvider`, `ErrEmailTaken`, `ErrCanonicalNameImmutable` and
`ErrInvalidContent` are programming-error-shaped sentinels (bad provider tag,
duplicate email at the storage layer, attempted rename of an immutable
canonical name, content that fails a structural check before format
validation) — they
currently surface as `Internal` if they ever escape to a handler. If you see
one of these in a client-facing error, it's worth filing as a bug: the
intent is that validation catches these before they reach the domain layer.

## etcd-compatible gRPC API errors

The etcd wire protocol doesn't use Connect codes — see
[Reference → etcd API compatibility](etcd-compatibility.md) for how these
same conditions surface as gRPC status codes / etcd error strings on port
`2379`.
