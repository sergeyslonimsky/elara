# Basic Auth Setup

On startup with `basic-auth`, Elara seeds a single local admin user from the
configured credentials, adds it to the system superadmin group, and marks it as
a protected system user (`User.System = true`, so it can never be deactivated or
deleted through the API). The username **must be email-shaped** — it is
normalized into the user's login identity and fails fast at startup otherwise.

## Environment

```bash
UI_AUTH_ENABLED=true
UI_AUTH_TYPE=basic-auth
UI_AUTH_BASICAUTH_USERNAME=admin@example.com
UI_AUTH_BASICAUTH_PASSWORD=change-me-now
# Plain HTTP locally? keep this false. Behind TLS in prod? set true.
UI_AUTH_SESSION_SECURECOOKIE=false
```

## Required first step: forced password change

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
