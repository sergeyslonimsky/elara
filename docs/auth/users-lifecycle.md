# Users & Groups lifecycle

This page covers what happens to a user (and the groups they belong to) over
time — deactivation, reactivation, and the system-protection flags. For the
permission model itself, see
[Auth & Access → RBAC, Groups & Roles](rbac-groups.md).

## User status

`UserStatus` (`internal/domain/user_status.go`) has two values:
`UserStatusActive` (`"active"`) and `UserStatusDeactivated` (`"deactivated"`).

- `Deactivate()` sets `Status = deactivated`.
- `Reactivate()` sets `Status = active`.

Both return `ErrSystemImmutable` for system users (see
[System-protected entities](#system-protected-entities) below) — a system
user can never be deactivated through the API.

## Deactivation is atomic and cascades to sessions

Deactivating a user and revoking their sessions happen in a **single
transaction** — the usecase wraps `user.Deactivate()` and
`SessionService.RevokeAllForUser` together
(`internal/usecase/user/service_deactivate.go`), so a deactivated user can
never be left with a still-valid session by a partial failure.

The auth interceptor enforces this on every request: after session
validation it checks `User.Status != Active` and rejects with
`ErrUserDeactivated` — a deactivated user cannot make any authenticated
request, even one made with a session id that would otherwise still be
valid. See [Sessions & Tokens → Enforcement on every request](sessions-tokens.md#enforcement-on-every-request).

## What deactivation does NOT affect

- **Tokens are not revoked.** A service token's permissions come only from
  its own fields, never from the user who issued it — deactivating,
  renaming, or deleting the issuer has no effect on tokens they created.
  See [Sessions & Tokens → Tokens](sessions-tokens.md#tokens-service-credentials-for-etcd-clients).
  Revoke a token explicitly via `TokenService.Revoke` if that's the intent.
- **Sessions do not auto-restore on reactivation.** Revoked sessions are
  gone permanently — a reactivated user must log in again.

## System-protected entities

- **`User.System = true`** — set by seed/bootstrap, never by the API. Blocks
  deactivation and deletion via `User.EnsureMutable()`, which returns
  `ErrSystemImmutable`. The bootstrap superadmin (basic-auth local user, or
  the OIDC `adminEmail` placeholder — see [OIDC Setup](oidc.md))
  carries this flag, as does the passthrough synthetic admin (see
  [Passthrough](passthrough.md)).
- **`Group.System = true`** — set on the `superadmin` group. Protected from
  deletion and rename by the same `EnsureMutable` pattern. Note that
  "systemness" is carried by the flag, not by the name: there is no reserved
  name prefix, and the group appears in the UI as plain `superadmin`.

## Deactivating/reactivating a user

Via the Web UI (Users page → user detail → Deactivate/Reactivate) or
`UserService.DeactivateUser` / `UserService.ReactivateUser` on the
ConnectRPC API. Authorization is `User:Write` (global) or derived from
group membership (target user is in a group you hold `Group:Write` on),
plus an anti-escalation check: the caller must hold every permission the
target currently has — you cannot deactivate someone with more access than
you.

Deactivate/Reactivate are **not** gated to a specific auth type (unlike
password reset and account deletion, which require basic-auth) — they work
the same way under basic-auth and OIDC. That distinction is the useful one: an
OIDC admin holding `User:Write` can deactivate a user even though reset and
delete are unavailable to them.

Under passthrough there is nothing to manage. With `ui.auth.enabled=false` the
user and group handlers are never mounted, so the RPCs return 404; with
`enabled=true` + `type=none` they are mounted but `CreateUser` is rejected with
`ErrFeatureNotAvailable`, and the UI hides the Users and Groups sections
outright (direct navigation redirects to the dashboard).
