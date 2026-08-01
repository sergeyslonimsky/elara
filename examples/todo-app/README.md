# Elara todo-app demo

A minimal stack that shows Elara managing configuration for three microservices
that consume it over the **etcd v3 protocol** — using the stock
`go.etcd.io/etcd/client/v3` library, unchanged.

```bash
docker compose up --build
# UI:  http://localhost:8080  (admin@elara.local / password)
```

Then follow the walkthrough in
[`docs/tutorial-todo-app.md`](../../docs/tutorial-todo-app.md).

## What's here

| Path | What it is |
|------|------------|
| `docker-compose.yml` | Elara + `api`, `worker`, `notifier`, and a one-shot `seed` container. |
| `services/` | A single Go module with the toy services. |
| `services/watcher/` | Shared helper: connect, read a key, watch for changes. |
| `services/cmd/{api,worker,notifier}/` | One `main.go` per service; each watches one key. |
| `services/cmd/seed/` | Writes the initial config keys via the etcd client. |

## Config keys

Each service reads and watches one key (`/{namespace}/{path}` addressing):

| Service | Key |
|---------|-----|
| `api` | `/production/api/limits.json` |
| `worker` | `/production/worker/settings.json` |
| `notifier` | `/production/notifier/config.json` |

## Notes

- **etcd client auth is disabled** in this demo (`CLIENT_AUTH_ENABLED=false`) so
  the services connect without a token. In production, leave it on and issue a
  service Token per client.
- Elara is **built from the repository root Dockerfile** so the demo tracks
  your checkout. To pin a published image instead, swap the `build:` block for
  the commented `image:` line in `docker-compose.yml`.
- The toy services fall back to sane defaults if a key doesn't exist yet, so the
  stack is resilient to startup ordering.
- Keys written over the raw etcd wire (by the services/seeder) don't create a
  namespace *record*. Create the `production` namespace once in the UI to manage
  those keys there — the tutorial covers this in Step 1.
