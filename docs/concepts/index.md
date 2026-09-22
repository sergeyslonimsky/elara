# Concepts

This section is a reference for Elara's core building blocks — what each one
is, the fields that matter, and the rules the server actually enforces. It is
not a walkthrough: for hands-on introductions see the
[Quickstart](../getting-started/quickstart.md) and the
[To-Do app tutorial](../getting-started/tutorial-todo-app.md).

Elara stores everything in a single [bbolt](https://github.com/etcd-io/bbolt)
file and exposes it three ways: a Web UI, a ConnectRPC API (both on port
`8080`), and an etcd-compatible gRPC API (port `2379`). The concepts in this
section are shared across all three surfaces:

- [Namespaces & Configs](namespaces-configs.md) — the two core entities every
  config lives inside.
- [Schema Validation](schema-validation.md) — JSON Schema enforcement per
  path pattern.
- [Bundles](bundles.md) — export/import snapshots for GitOps-style workflows.
- [Watch & Revisions](watch-revisions.md) — the global revision counter and
  the etcd watch model built on top of it.
- [Webhooks](webhooks.md) — HTTP delivery of config-change notifications.
- [Clients & Observability](clients-observability.md) — who's connected to
  the etcd-compatible API right now, and what they're reading.

For the permissions model (who can do what), see
[Auth & Access → RBAC, Groups & Roles](../auth/rbac-groups.md).
