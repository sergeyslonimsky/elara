# Client Auth (etcd)

Token enforcement on the etcd gRPC port is gated by `client.auth.enabled`
(env `CLIENT_AUTH_ENABLED`) — independent of UI auth (see
[Auth & Access overview](index.md)):

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

When enabled, clients authenticate with a **token** — see
[Sessions & Tokens](sessions-tokens.md#tokens-service-credentials-for-etcd-clients)
for how to issue one and use it.
