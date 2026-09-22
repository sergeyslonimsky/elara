# RBAC, Groups & Roles

Elara uses Casbin RBAC with one hard invariant: **users get permissions only
through group membership**. There is no API to grant a permission directly to a
user. The rationale is documented in
[ADR 0002 — Groups-only RBAC](../adr/0002-groups-only-rbac.md); this page is
the practical model and how-to (`internal/domain/rbac.go`).

## Permissions are tuples, not canned roles

A group holds a set of `Permission{Object, Action, Domain}` grants, assigned
explicitly through the API or UI. There is no fixed "reader gets X, writer gets
Y" expansion table on the web side — you compose the exact objects/actions/scopes
a group needs.

- **Objects**: `namespace`, `token`, `client`, `dashboard`, `user`, `group`,
  `policy`, `webhook`, and `*` (all). Note that `namespace` covers *all*
  namespace-scoped content — configs, schemas, and export/import included; there
  is no separate object for those.
- **Actions**: `read`, `write`, `create`, `delete`, and `*` (all). `write`
  implies `read` (you can't edit what you can't read), and `*` matches any
  action.
- **Domain (scope)**: the scope a grant applies to — a namespace
  (`namespace:<name>`), a group (`group:<name>`), or `*` (global / every
  namespace).

**Namespace-scoped vs global.** A grant like `namespace / write / namespace:prod`
lets the group write configs in `prod` only. The same grant with domain `*`
applies to every namespace. Group membership itself is always global.

## The role names

`admin`, `writer`, and `reader` exist as constants. `admin` is the wildcard
role: the seeded system superadmin group is granted `* / * / *` (all objects,
all actions, all namespaces) as a break-glass policy. `reader` and `writer` are
used primarily as **etcd service-token roles** — a token carries a role plus a
namespace list, where `reader` is read-only and `writer` additionally permits
writes within its namespaces (`internal/handler/etcdv3/access.go`). Service
tokens are independent credentials — see
[Sessions & Tokens](sessions-tokens.md).

For the Casbin-facing role/action semantics used by groups specifically:

| Role | Object / Action semantics |
|------|---------------------------|
| `admin` | All actions (`*`) on all objects (`*`) — full control |
| `writer` | `write` on content (write **implies** read) |
| `reader` | `read` on content |

The system superadmin group carries the single break-glass rule
`(group:superadmin, *, *, *)` — all objects, all actions, all namespaces — and
its members are global admins.

## Practical flow: grant someone admin over a namespace (or globally)

1. **Create a group** (Web UI *Groups* → *Create*, or `GroupService.Create`).
   You can attach initial permissions and members atomically at creation time.
2. **Assign the group a permission** in a domain: pick the object/action and set
   the domain to a specific namespace (`namespace:prod`) or `*` for all
   namespaces. For a namespace admin, grant all-actions-on-all-objects scoped to
   that namespace; for a platform admin, scope it to `*` (or just add them to
   the built-in superadmin group).
3. **Add the user to the group.** Membership is what actually confers the
   permissions.

Anti-escalation is enforced throughout: you cannot grant a group (or its
members) any permission you do not yourself already hold. The bootstrap admin
(basic-auth local user, or the OIDC `adminEmail` identity) starts in the
superadmin group and can therefore set up all other groups.
