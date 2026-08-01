# Tutorial: manage configs for a 3-service todo app

This walkthrough runs a small stack — Elara plus three toy microservices — and
takes you through the four things that make Elara more than "etcd with a UI":

1. Start the stack and watch the services read their config from Elara.
2. Change a limit in the Web UI and watch a service pick it up **live**, with
   no restart, over the etcd watch protocol.
3. Attach a JSON Schema to a key and see an invalid value get **rejected**
   before it is ever stored.
4. Export a config bundle, edit it in Git, and import it back — a GitOps
   round-trip.

The three services (`api`, `worker`, `notifier`) are deliberately tiny. Each is
a ~40-line Go program that uses the **stock `go.etcd.io/etcd/client/v3`
library** — unchanged — to read one config key on startup and watch it for
changes. That's the whole point: your existing etcd clients talk to Elara with
zero code changes.

Everything for this tutorial lives in
[`examples/todo-app/`](../examples/todo-app/).

---

## Prerequisites

- Docker with the Compose plugin (`docker compose version` should work).
- Ports `8080` (Web UI) and `2379` (etcd gRPC) free on your machine.

No Go toolchain is needed on your host — everything builds inside containers.

---

## Step 1 — Start the stack

From the repository root:

```bash
cd examples/todo-app
docker compose up --build
```

The first build compiles the Elara image (frontend + Go binary) and the three
toy services, so it takes a minute or two. When it settles you'll see the
services connect and load their config. A one-shot `seed` container writes the
initial keys, so the services show real values right away:

```
api-1       | [api] loaded /production/api/limits.json = {"max_todos_per_user":100,"rate_limit_per_min":60}
api-1       | [api] watching /production/api/limits.json for changes...
worker-1    | [worker] loaded /production/worker/settings.json = {"batch_size":50,"poll_interval_seconds":5}
notifier-1  | [notifier] loaded /production/notifier/config.json = {"channel":"email","digest_enabled":true}
```

If a service starts before Elara is ready it retries automatically, so a few
"cannot reach Elara … retrying" lines at the top are normal.

Now open the Web UI at <http://localhost:8080> and log in:

- **Username:** `admin@elara.local`
- **Password:** `password`

### Register the namespace in the UI

The services and the seeder address config by `namespace/path` and talk to
Elara over the raw etcd wire, which doesn't create a namespace *record*. To
manage those keys in the UI, create the matching namespace once:

1. Create a namespace named **`production`** in the Web UI.

The three keys the seeder already wrote (`api/limits.json`,
`worker/settings.json`, `notifier/config.json`) now appear inside it — creating
the namespace record doesn't touch the config data that's already there.

> **Tip:** keep the `docker compose up` terminal visible. The service logs are
> how you'll *see* config changes propagate in the next steps.

---

## Step 2 — Change a limit and watch it propagate live

The `api` service reads `/production/api/limits.json`. Let's raise its todo
limit and watch the running service react without a restart.

1. In the Web UI, open the **`production`** namespace.
2. Open the config **`api/limits.json`**.
3. Change `max_todos_per_user` from `100` to `500` and save.

Within a second, the `api` service logs the new value in your `docker compose`
terminal:

```
api-1  | [api] config changed: /production/api/limits.json = {"max_todos_per_user":500,"rate_limit_per_min":60} (revision 6)
```

That change travelled from the Web UI → Elara → the etcd **watch** stream the
`api` service opened at startup. The service used the ordinary
`clientv3.Watch(...)` API — the same call it would make against a real etcd
cluster.

You can prove the drop-in compatibility from your host too, if you have
`etcdctl` installed (client auth is disabled in this demo, so no token is
needed):

```bash
etcdctl --endpoints=localhost:2379 get /production/api/limits.json
etcdctl --endpoints=localhost:2379 put /production/worker/settings.json '{"batch_size":100,"poll_interval_seconds":2}'
```

Watch the `worker` service log the change you just made with `etcdctl`.

---

## Step 3 — Attach a JSON Schema and reject an invalid value

Elara can validate config content against a JSON Schema **before** it is
written. Invalid values are rejected up front instead of quietly breaking a
consumer.

> **Where it applies:** schema validation runs on the config-management path —
> edits made through the Web UI and the ConnectRPC API. It is what protects a
> human (or a CI job) from saving a broken config. (Raw etcd `Put` calls are
> the low-level wire path and are not schema-checked, so keep management edits
> going through the UI/API where the guardrails live.)

1. In the Web UI, open the **`production`** namespace.
2. Use the namespace's **Attach Schema** action. Fill in:
   - **Path Pattern:** `/api/**` (this glob matches `api/limits.json`).
   - **JSON Schema:** paste the schema below.

   ```json
   {
     "type": "object",
     "properties": {
       "max_todos_per_user": { "type": "integer", "minimum": 1, "maximum": 1000 },
       "rate_limit_per_min": { "type": "integer", "minimum": 1 }
     },
     "required": ["max_todos_per_user", "rate_limit_per_min"],
     "additionalProperties": false
   }
   ```

3. Now open `api/limits.json` and try to save an invalid value, for example:

   ```json
   { "max_todos_per_user": 5000, "rate_limit_per_min": 60 }
   ```

   `5000` exceeds the schema's `maximum` of `1000`, so Elara rejects the save
   and the UI shows a schema-validation error. The stored value — and what the
   `api` service sees — is unchanged.

4. Fix the value (e.g. `750`) and save. This time it succeeds, and the `api`
   service logs the update.

Try another invalid case to get a feel for it: change `max_todos_per_user` to
the string `"lots"` and save — the `integer` type constraint rejects it.

---

## Step 4 — Bundle export → edit in Git → import back

Elara can round-trip a namespace to a YAML or JSON bundle, so you can review
config changes in a pull request and apply them back — a lightweight GitOps
flow.

**Export.** In the Web UI, open the `production` namespace and use its
**Export** action. Choose the bundle format (**YAML** or **JSON**) and
download. You'll get a file like `production-export.yaml` containing every key
in the namespace:

```yaml
namespace: production
configs:
  - path: /api/limits.json
    content: '{"max_todos_per_user":750,"rate_limit_per_min":60}'
    format: json
  - path: /notifier/config.json
    content: '{"channel":"email","digest_enabled":true}'
    format: json
  - path: /worker/settings.json
    content: '{"batch_size":100,"poll_interval_seconds":2}'
    format: json
```

**Edit in Git.** Commit that file to a repo and change a value in a normal
editor — say, flip the notifier channel:

```yaml
  - path: /notifier/config.json
    content: '{"channel":"slack","digest_enabled":true}'
    format: json
```

**Import back.** In the Web UI, use the **Import Configs** action, upload the
edited bundle, and choose how to resolve conflicts (overwrite existing keys to
apply your edit). You can do a dry run first to preview what would change.

Once imported, the `notifier` service logs the new value it picked up over its
watch:

```
notifier-1  | [notifier] config changed: /production/notifier/config.json = {"channel":"slack","digest_enabled":true} (revision 9)
```

Your Git history is now the audit trail for that change, and Elara enforced it
against any attached schemas on the way in.

---

## Tear down

```bash
docker compose down          # stop the stack, keep the data volume
docker compose down -v       # also delete Elara's stored data
```

---

## How it fits together

```
Web UI / ConnectRPC ─┐
                     ├─→  Elara  ─(bbolt)
etcdctl / your app ──┘      │
                            └─→ etcd v3 watch stream ──→ api / worker / notifier
```

- The toy services use the **unmodified** `go.etcd.io/etcd/client/v3` library.
  Keys are addressed as `/{namespace}/{path}`, e.g.
  `/production/api/limits.json`.
- A change made in the UI, via the ConnectRPC API, or via `etcdctl` all land in
  the same store and fan out to every watcher.
- The management path (UI/API) adds the guardrails bare etcd doesn't have:
  JSON Schema validation, RBAC, audit history, and GitOps bundles.

To go deeper, read the service code in
[`examples/todo-app/services/`](../examples/todo-app/services/) — it's about a
page of Go per service.
