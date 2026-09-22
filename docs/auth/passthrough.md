# Passthrough / No-Auth Mode

!!! danger "Development only"
    Passthrough mode performs **no authentication at all**. Every request to the
    UI/ConnectRPC API is treated as a superadmin. Never expose a passthrough
    instance on a network anyone else can reach.

There are two ways in, and they are equivalent:

```bash
# 1. Leave UI auth off entirely. ui.auth.type is ignored and forced to "none".
UI_AUTH_ENABLED=false

# 2. Turn the auth surface on with no identity provider behind it.
UI_AUTH_ENABLED=true
UI_AUTH_TYPE=none
```

The second form is what the Helm chart produces when `type` is left at `none`
with auth enabled, and it is fully supported — not a misconfiguration.

How it works: at startup `BootstrapPassthrough` seeds a synthetic system user
(`local-admin@elara.internal`, `User.System = true`) into the superadmin group.
The auth interceptor runs in a bypass mode (`skipPermissions=true`) — any
request, authenticated or not, is rewritten with this synthetic admin context,
so enforcement uniformly resolves to superadmin. There is no login screen and no
session to obtain. A one-shot `WARN` is logged at startup so the bypass is
grep-able in production-shaped logs.

Because there is no real principal, the authorization PDP must also run with
permissions skipped. The interceptor bypass and the PDP skip are wired in
lockstep from a single decision — the resolved **auth type**, not the
`ui.auth.enabled` flag — so you cannot get a half-open state through config
alone. Keying it off `enabled` was a real bug: in the `enabled=true` +
`type=none` form above, the interceptor injected its synthetic admin while the
PDP kept enforcing against a principal that has no policy, and every request
was denied.
