# 2. Groups-only RBAC: users get permissions solely through group membership

- Status: accepted
- Date: 2026-08-01
- Deciders: Elara maintainers

## Context and Problem Statement

Elara authorizes every request through a Casbin RBAC model over the tuple
`(subject, domain, object, action)`. A permission is a `p`-rule granting a
*role* a capability in a *domain* (namespace or group scope); a `g`-rule binds a
subject to something. The open question was: **how does a concrete user acquire
a role?** Either the user is bound directly to a role, or the user is bound to a
group and the group is bound to a role.

Direct `user → role` grants are the conventional escape hatch, but they create
permission drift: one-off elevations accumulate, are invisible next to the
group-based grants they duplicate, and are painful to audit ("who is admin on
`prod`?" requires scanning both users and groups). They also do not compose with
SSO/OIDC group synchronization, where the identity provider is authoritative over
*groups*, not over per-user role bindings.

## Decision Drivers

- Auditability: a single, uniform place to answer "who can do what where".
- SSO/OIDC-friendliness: IdP group claims map cleanly onto our subjects.
- One mental model: no second, competing grant path to reason about.
- Fail-closed, provable invariants over convenience.

## Considered Options

1. **Groups-only** — users bind to groups, groups bind to roles. No API to bind a
   user to a role.
2. **Groups + direct grants** — groups-only by default, with a direct
   `user → role` escape hatch for one-offs.
3. **Per-resource ACLs** — drop role indirection; attach subjects directly to
   resources (Zanzibar/OpenFGA-style relations).

## Decision Outcome

Chosen option: **"Groups-only"**, enforced structurally — there is **no API
surface** that can write a `user → role` grant. The only `g`-rules the system
ever writes are:

```
g, <user-UUID>, group:<name>, *              # membership, global (MembershipDomain)
g, group:<name>, <role>, <namespace-or-*>    # the group's role in a domain
```

Users appear as bare UUIDs, groups as `group:<name>` (`domain.IsUserSubject` /
`IsGroupSubject`). Casbin resolves the two-hop chain natively: the enforcer
registers `AddNamedDomainMatchingFunc("g", "keyMatch", util.KeyMatch)`
(`internal/service/auth/casbin/enforcer.go`), so a membership rule stored with
domain `*` matches *any* queried domain during recursive role resolution. A user
in `group:devs` (global membership) inherits `admin` on `prod` purely through
recursion — **no application-side fan-out**, and membership never has to be
re-materialized per namespace.

The **one** intentional exception proves the rule by how carefully it is
special-cased. `AdminBootstrap` (`internal/service/auth/bootstrap.go`) seeds a
`superadmin` group with a break-glass wildcard `p`-rule
(`group:superadmin, *, *, *`) and adds the bootstrap admin via
`EnsureMember` — i.e. it grants root by **writing a membership rule, not a
role**, going through the same `group:` path as everyone else. The seed user and
group carry `User.System` / `Group.System = true`, protected from mutation via
`EnsureMutable()` / `ErrSystemImmutable`. Even "no-auth" passthrough mode injects
a synthetic user *into the superadmin group* rather than short-circuiting
enforcement. There is no code path where a role reaches a user except through a
group.

This decision was realized in `d07a399` (RBAC extracted into `domain/rbac.go` +
`service/auth/casbin`), `29e7940` and `1076fb7` (groups/users model + CASL UI
mapping). `e89ea75` hardens the invariant's integrity, keeping the in-memory
Casbin cache in sync with bbolt when a user or group is deleted so no orphan
membership rule can silently linger.

## Consequences

Positive:

- **Auditability**: effective rights come from one call,
  `GetImplicitPermissionsForUser`; group membership is the single lever.
- **SSO/OIDC-friendly**: IdP group claims translate directly into membership
  `g`-rules; deprovisioning a group revokes everyone in it at once.
- **Simpler model**: one grant path, no reconciliation between two systems, and a
  structurally impossible class of "forgotten direct grant" bugs.

Negative:

- A group must exist even to grant a single user a one-off permission — extra
  indirection for the simplest cases.
- Renaming/refactoring groups requires care because they are the *only* handle on
  access; there is no per-user override to lean on.

## Pros and Cons of the Options

### Groups-only

- Good: uniform audit surface; native Casbin recursion; SSO-aligned.
- Bad: mandatory group for one-off grants.

### Groups + direct grants

- Good: convenient one-off elevations.
- Bad: reintroduces drift, dual audit paths, and SSO impedance — the exact
  problems being solved.

### Per-resource ACLs

- Good: fine-grained, relation-level control.
- Bad: heavier model, no role abstraction, larger blast radius than Casbin RBAC
  needs to carry today (an explicit non-goal in `plans/EL-4`).
