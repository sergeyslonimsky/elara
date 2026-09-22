# Passthrough / No-Auth Mode

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
