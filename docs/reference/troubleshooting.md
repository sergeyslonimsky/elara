# Troubleshooting & FAQ

## Startup

### `load container: open bbolt database: timeout`

Another process already holds the exclusive lock on the bbolt file at
`CONFIG_DATA_PATH` (default `~/.elara/data/elara.db` for a bare binary,
`/var/lib/elara/elara.db` in the container image). Elara waits up to 1 second
for the lock before giving up. This is by design: **only one Elara instance
may run against a given data file at a time** — see
[Kubernetes → `replicaCount`](../deployment/kubernetes.md#values-reference).

Fix: stop the other instance, or point `CONFIG_DATA_PATH` at a different
directory if you actually intend to run two independent instances.

### `basic-auth requires ui.auth.basicAuth.username to be set` / `...password to be set`

`UI_AUTH_TYPE=basic-auth` but `UI_AUTH_BASICAUTH_USERNAME` or
`UI_AUTH_BASICAUTH_PASSWORD` is empty. See
[Basic Auth Setup](../auth/basic.md#environment).

### `oidc requires ui.auth.oidc.adminEmail to be set`

`UI_AUTH_TYPE=oidc` but `UI_AUTH_OIDC_ADMINEMAIL` is empty — Elara needs it to
know who to bootstrap as the first superadmin. See
[OIDC Setup](../auth/oidc.md#environment).

## Login and permissions

### "password change required" blocks every API call after basic-auth login

Expected on first login with the bootstrap admin credentials. Call
`ProfileService.ChangePassword` — see
[Basic Auth Setup → Required first step](../auth/basic.md#required-first-step-forced-password-change).
Only `ProfileService/{ChangePassword,Me,Logout}` work until you do.

### Logged in but the sidebar only shows Dashboard

Classic symptom of a fresh bbolt file where Casbin's superadmin wildcard
policy hasn't propagated into the in-memory enforcer yet — should self-resolve
after the first successful boot, since `main.go`'s startup sequence reloads
the policy right after bootstrap. If it persists across restarts, check that
`ui.auth.enabled` and the auth type match what you expect
(`GetCapabilities`/`GetAuthInfo` in the browser network tab will show what the
server thinks is configured) and that the logged-in identity is actually a
member of the superadmin group.

### Login works over HTTP but the session doesn't stick (redirects back to login)

`UI_AUTH_SESSION_SECURECOOKIE=true` marks the cookie `Secure`, and browsers
silently drop `Secure` cookies over plain HTTP (RFC 6265). Set it `false` for
local/plain-HTTP setups, `true` only behind TLS. See
[Auth & Access → `secureCookie` warning](../auth/index.md#configuration-keys).

### An OIDC user gets `PermissionDenied` / `identity not provisioned`

Elara does not JIT-provision OIDC users — only the configured
`adminEmail` bootstraps automatically. Every other user must be created in
Elara first, then linked on their first verified OIDC login. See
[OIDC Setup](../auth/oidc.md).

### A write over the etcd-compatible API isn't rejected by a schema I attached

Expected today — schema validation runs on the Web UI/ConnectRPC write path,
not yet on etcd `Put`. See the warning in
[Schema Validation](../concepts/schema-validation.md) and
[etcd API compatibility → KV](../reference/etcd-compatibility.md#supported-rpcs).

## Backup & Restore

Elara does not yet expose an online backup RPC (`Maintenance/Snapshot` is
unimplemented — see
[etcd API compatibility](etcd-compatibility.md#supported-rpcs)). Until it
does, back up by copying the bbolt file while the process is **stopped**:

```bash
# stop elara, then:
cp /var/lib/elara/elara.db /backup/elara-$(date +%F).db
# restart elara
```

Copying the file while Elara is running is unsafe — bbolt's on-disk format
requires a consistent snapshot, and a live file copy can capture a
torn/inconsistent write. If you need online backups, take periodic
[Bundle exports](../concepts/bundles.md) instead (`TransferService`) — those
are safe to run against a live instance, though they capture config content,
not the full store (RBAC policy, sessions, tokens, webhook history).

## General

### General error reference

For what a given error code/message means across both APIs, see
[Reference → Errors catalog](errors-catalog.md).

### Where does Elara store its data?

A single bbolt file — see `CONFIG_DATA_PATH` in
[Configuration reference](../deployment/configuration.md#server-data).
No external database, message broker, or cluster.
