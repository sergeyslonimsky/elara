# etcd-watcher

Demo client for verifying the Elara connected-clients UI.

## What it does

1. Connects to a running Elara instance over its etcd-compatible gRPC API
   (`localhost:2379` by default)
2. Sends rich identity headers (`x-client-name`, `x-client-version`,
   `x-client-k8s-namespace`, `x-client-k8s-pod`, `x-client-k8s-node`,
   `x-client-instance-id`) so it shows up nicely under `/clients`
3. Opens a long-running `Watch` on a key prefix
4. Periodically does `Put` + `Get` to drive the per-method counters and
   trigger watch events visible in the activity log

## Run

```bash
cd examples/etcd-watcher
go mod tidy
go run .
```

Then open `http://localhost:8080/clients` — you should see one row named
`order-service v1.2.3` with a green pulse, growing request counters, and
`Watches: 1`.

Click into the row to see the detail page: KPI cards live-update, the
activity log streams in real time, the method donut fills in.

## Multiple clients

Run a second instance with a different identity to compare in the UI:

```bash
go run . --name payment-service --pod payment-abc-12 --interval 2s
go run . --name order-service   --pod order-7d8c-x4 --interval 7s --instance-id pod-xyz
```

## Demonstrating the Watches tab

The Watches tab shows what the client is subscribed to. By default this demo
opens a *prefix* watch on `/default/`, which the UI renders as
`[prefix] default /` (i.e. "all configs in the `default` namespace").

To verify the **single-key watch** rendering — which shows the exact config
file path:

```bash
go run . --watch-mode key --watch-key /default/demo/heartbeat.json
```

The Watches tab will show: `[key] default /demo/heartbeat.json`.

To verify the **explicit range** rendering:

```bash
go run . --watch-mode key --name r1 --watch-key /default/foo  # single config
go run . --watch-mode range --name r2 \
   --watch-key /default/a --watch-end /default/m              # alphabetical range
```

To run with watch disabled (e.g. to populate the Counters tab without showing
up in Watches):

```bash
go run . --watch-mode none --interval 1s
```

## Demonstrating the Errors tab

Use `--errors-every` to make every Nth heartbeat fail on purpose. The failing
Put uses `WithIgnoreValue()` which our server returns `Unimplemented` for —
that surfaces as a real RPC error in the UI.

```bash
# Every 3rd request fails — so error counter grows steadily
go run . --errors-every 3 --interval 1s
```

The Errors tab will fill in with rows like:
`Put · /default/demo/heartbeat.json · rpc error: code = Unimplemented desc = ignore_value is not supported`

The tab label will show `Errors (N)` in red where N is the total error count.

## Test disconnect

`Ctrl+C` the watcher. Within ~2s the UI will:

- Move the row from the `Active` tab to the `History` tab
- If you were on the detail page, the badge flips to `● Disconnected` and
  the activity log freezes
- The next instance launched with the same `--name` + `--k8s-ns` will see
  the previous run in its `Sessions` tab

## Flags

| Flag | Default | Purpose |
|------|---------|---------|
| `--endpoint` | `localhost:2379` | Elara etcd-compatible gRPC endpoint |
| `--name` | `order-service` | `x-client-name` header |
| `--version` | `1.2.3` | `x-client-version` header |
| `--k8s-ns` | `production` | `x-client-k8s-namespace` header |
| `--pod` | `order-7d8c-x4k2` | `x-client-k8s-pod` header |
| `--k8s-node` | `gke-node-abc` | `x-client-k8s-node` header |
| `--instance-id` | `instance-uuid-1` | `x-client-instance-id` header |
| `--watch-mode` | `prefix` | one of `prefix`, `key`, `range`, `none` |
| `--watch-key` | `/default/` | key (or start key) to watch |
| `--watch-end` | (empty) | end key — required for `--watch-mode=range` |
| `--write-path` | `/default/demo/heartbeat.json` | etcd key written every tick |
| `--read-path` | (= write-path) | etcd key read every tick |
| `--interval` | `5s` | tick interval; `0` disables writes (watch-only) |
| `--errors-every` | `0` | every Nth request fails on purpose; `0` disables |
