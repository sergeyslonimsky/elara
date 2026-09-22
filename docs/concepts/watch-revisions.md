# Watch & Revisions

## Global Revision

Elara maintains a single **monotonic revision counter** that increments on every
config write across all namespaces — the etcd `mod_revision` model. Each write
stamps the config's `Revision` (and, on create, `CreateRevision`) with the
counter's value and records a history entry at that revision.

This counter is what enables point-in-time reads and etcd
watch-from-revision, and it surfaces on the dashboard as the **Global Revision**
KPI (`internal/usecase/dashboard/service_stats.go`, `CurrentRevision`). Because
it is global, its value reflects total write activity across the whole store,
not any single namespace.

## Watch

Elara implements the etcd v3 `Watch` RPC (`internal/handler/etcdv3/watch_server.go`)
as a bidirectional gRPC stream: a client opens one stream and multiplexes any
number of watches over it by sending `WatchCreateRequest`/`WatchCancelRequest`
messages, each identified by a client-chosen `WatchId`.

- **Live watches** — omit `StartRevision` (or set it to `0`) and the watch
  fires only for writes that happen from the moment it's created onward.
- **Resumable / from-revision watches** — set `StartRevision` to a past
  revision and Elara first **replays history** from that revision up through
  the current one, then transitions to live delivery — so a client that was
  disconnected can reconnect with its last-seen revision and not miss any
  writes, the same resume pattern real etcd clients rely on.
- **`PrevKv`** — set the flag on `WatchCreateRequest` to have each event
  include the value as it was *before* the write, not just the new value.
- Every event is namespace/prefix filtered server-side against the watch's
  key range — clients only receive events for keys they actually asked about.

The connected-clients monitor (see
[Clients & Observability](clients-observability.md)) tracks each active watch
per connection, including its key range, start revision, and flags — visible
live in the Web UI's Clients view.

!!! note "Watching from outside the etcd API"
    Webhooks (see [Webhooks](webhooks.md)) and the Web UI's live views consume
    the same underlying domain-level change publisher
    (`internal/transport/watch/publisher.go`) that feeds etcd watches — a
    write triggers all three consumers uniformly, regardless of which surface
    made the write.
