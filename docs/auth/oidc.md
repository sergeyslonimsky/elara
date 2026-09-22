# OIDC Setup

Under OIDC, Elara does **not** self-provision arbitrary users
(no JIT provisioning): an unknown identity is rejected with
`ErrIdentityNotProvisioned`. The one exception is the configured admin, which is
bootstrapped specially.

## Environment

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

## How the admin bootstrap actually works

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
