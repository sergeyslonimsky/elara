# Quickstart

This walkthrough takes about **5 minutes**. You'll start Elara from a single
Docker command, tour the seeded demo data, and make your first configuration
change through the Web UI — watching schema validation reject a bad value along
the way.

## 1. Run Elara

There's no published `:demo` image yet — build the demo variant locally from
the repo, which seeds sample data via `DEMO_MODE`:

```bash
git clone https://github.com/sergeyslonimsky/elara.git
cd elara
docker build --build-arg DEMO_MODE=true -t elara:demo .
docker run --rm -p 8080:8080 -p 2379:2379 elara:demo
```

That's the whole install. Elara is a single binary backed by one
[bbolt](https://github.com/etcd-io/bbolt) file — there's no external database,
message broker, or cluster to stand up.

Two ports are now listening:

| Port   | Surface                                            |
|--------|----------------------------------------------------|
| `8080` | Web UI + ConnectRPC API (HTTP/2)                    |
| `2379` | etcd-compatible gRPC API (drop-in for etcd v3)     |

Open <http://localhost:8080> in your browser. On first launch demo mode
greets you with a welcome modal explaining that you're looking at sample data —
dismiss it to reach the dashboard.

!!! note "Prefer a clean instance?"
    Build without `--build-arg DEMO_MODE=true` (or run the binary directly via
    `go run ./cmd/service`) and the instance starts empty. Create a namespace
    and a config from the UI (or with `etcdctl`, see below) and the rest of
    this tour still applies.

!!! note "Don't want Docker?"
    Every [release](https://github.com/sergeyslonimsky/elara/releases) also
    publishes prebuilt binaries for macOS and Linux — download, extract, and
    run `./elara`. See [Configuration](configuration.md#config-file-for-local-installs)
    for its `~/.elara` data/config defaults.

## 2. Tour the seeded demo data

The demo image ships with three namespaces, roughly ten keys, JSON Schemas
attached to a few path patterns, and a set of simulated Kubernetes pod
consumers so the observability views have something to show.

Start on the **Dashboard**. It gives you the cluster-wide picture at a glance:

- Total namespaces, configs, active clients, and the current **global
  revision** (a monotonic, etcd-style counter that bumps on every write).
- The last 20 config changes.
- A per-namespace config count.

Now click through the left navigation:

- **Namespaces** — three logical groups are seeded: `production`, `staging`,
  and `dev`. Namespaces are just containers for configs; a namespace can't be
  deleted while it still holds any.
- **Configs** — a directory-style browser. Open `production` and drill into the
  seeded keys, for example `production/api/limits.json`. The editor is
  format-aware (JSON / YAML / raw) and every key carries **version history** —
  each edit is retained and retrievable by revision.
- **Clients** — the live list of connected etcd-compatible clients. The demo
  seeds simulated k8s pod consumers here so you can see which workloads are
  reading configuration and at which revision, without wiring up a real
  cluster.

Take a minute to open a config's **history** tab and a side-by-side diff of two
revisions — this is the audit trail Elara keeps for every change.

## 3. Make your first change through the UI

Let's edit a value and watch Elara enforce a schema.

1. Go to **Configs → `production`** and open `api/limits.json`. In the demo
   seed this file has a JSON Schema attached to its path pattern (visible on the
   config's **Schema** tab), so writes are validated before they're stored.
2. Click **Edit**. Change a numeric field — say bump a rate limit from `100` to
   `250` — and **Save**. The write succeeds, the **global revision increments**
   (watch the dashboard counter), and a new entry appears in the change feed.
3. Now break it on purpose. Edit the same file and set that field to a string,
   e.g. `"lots"`, or delete a required property. **Save** again.

   The write is **rejected before storage**. Elara returns a validation error
   listing the exact failing path, the message, and the JSON Schema keyword
   that failed — the config on disk is untouched. This is the core guardrail:
   invalid configuration never reaches the services that consume it.

4. Fix the value back to a valid number and save once more to confirm the write
   goes through.

### The same change from an etcd client

Every UI edit is just a write against the same store your services read from.
Any etcd v3 client can do it too:

```bash
export ETCDCTL_API=3
export ETCDCTL_ENDPOINTS=localhost:2379

# Read the seeded value
etcdctl get /production/api/limits.json

# Watch the key while you edit it in the UI — updates stream live
etcdctl watch /production/api/limits.json
```

Leave the `watch` running and repeat step 3 in the browser: valid writes show
up on the stream immediately, and rejected ones never appear — because they
were never stored.

## Next steps

- Read the [Architecture](architecture.md) overview to understand how a request
  flows from the UI or an etcd client down to the bbolt file.
- Turn on **authentication** (basic-auth or OIDC) and explore the groups-based
  RBAC model — see [ADR 0002 — Groups-only RBAC](adr/0002-groups-only-rbac.md).
- Point one of your own services at `localhost:2379` with its existing etcd v3
  client. No code changes: Elara speaks the etcd v3 wire protocol.

## Developing Elara itself

Want to build from source rather than run the image? Prerequisites are the Go
version pinned in [`go.mod`](https://github.com/sergeyslonimsky/elara/blob/master/go.mod),
Node.js 20+, `buf`, and `golangci-lint`.

```bash
git clone https://github.com/sergeyslonimsky/elara.git
cd elara

# Build the frontend first — web/embed.go embeds web/dist at compile time
cd web && npm install && npm run build && cd ..

# Run the service
go run ./cmd/service

# Tests (race detector on) and lint
make test
make lint
```

For live frontend reload, run the backend in one terminal and
`cd web && npm run dev` (<http://localhost:3000>) in another. The full
contributor guide is in
[CONTRIBUTING.md](https://github.com/sergeyslonimsky/elara/blob/master/CONTRIBUTING.md).
