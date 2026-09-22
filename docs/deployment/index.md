# Deployment

How to run Elara somewhere other than your laptop. If you just want to try it,
start with [Quickstart](../getting-started/quickstart.md) instead.

- **[Docker](docker.md)** — a single container with a persistent volume. The
  smallest real deployment, and the one the other two are built on.
- **[Kubernetes (Helm)](kubernetes.md)** — the chart, its `values.yaml`
  reference, ingress, gRPC exposure, and persistence.
- **[Configuration reference](configuration.md)** — every environment variable
  and config key, with defaults. Applies to all three install routes.

Two constraints shape every deployment:

**One instance at a time.** bbolt holds an exclusive lock on the data file, so
Elara does not scale horizontally — the Helm chart pins `replicaCount` to `1`.
Raft-based HA is on the roadmap; until then, a second replica will fail to
start rather than corrupt anything.

**Auth is off by default.** A fresh instance is a passthrough instance: every
request is treated as superadmin. Turn on authentication before exposing one
beyond your own machine — see [Auth & Access](../auth/index.md).
