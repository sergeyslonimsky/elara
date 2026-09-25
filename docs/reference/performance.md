# Performance baseline

This page records what Elara costs per operation, measured by the benchmarks
that live beside the code. It exists so that a performance change is something
that gets noticed rather than discovered later.

Treat the absolute numbers as indicative of one machine. What travels between
machines is the *shape*: the ratios between operations, how a cost scales with
size, and which line item dominates.

## Running them

```bash
cd web && npm run build   # web/embed.go needs web/dist/ to exist
make bench                # full run, 6 samples, writes benchmarks.txt
make bench-quick          # smoke run: do the benchmarks still work
```

To compare a change against its starting point:

```bash
git stash && make bench && mv benchmarks.txt base.txt && git stash pop
make bench && make bench-compare BASE=base.txt
```

CI does the same automatically on every pull request and prints the comparison
to the job summary. It is informational: on a shared runner benchstat reports
real regressions and imaginary ones in roughly equal measure, so it does not
gate the merge. The job does fail if a benchmark stops compiling or starts
erroring, which is what keeps the suite from quietly rotting.

## Two durability modes

bbolt calls `fsync` on every commit. That single syscall costs ~8 ms here —
roughly a hundred times more than everything else a write does — so it hides
whatever is being measured.

Benchmarks therefore run with `NoSync` by default, and the ones whose name ends
in `_Durable` keep the real fsync behaviour. The mode is part of the benchmark
name deliberately: benchstat matches on names, so it cannot silently diff one
mode against the other.

Read it this way: `_Durable` numbers are the latency a client observes. The
rest are the cost of the code, which is the part a change can actually move.

## Storage — etcd-compatible KV path

`internal/storage/bbolt/config/etcd.go`

| Operation | Time | Allocations |
|---|---|---|
| `PutKey` new key | 86 µs | 186 |
| `PutKey` new key, **durable** | 8.2 ms | 154 |
| `PutKey` overwrite | 66 µs | 159 |
| `PutKey` contended (8 writers, one key) | 63 µs | 159 |
| `RangeQuery` single key | 2.9 µs | 26 |
| `RangeQuery` prefix, 10 keys | 20 µs | 85 |
| `RangeQuery` prefix, 100 keys | 189 µs | 728 |
| `RangeQuery` prefix, 1000 keys | 1.9 ms | 7032 |
| `RangeQuery` keys-only, 100 keys | 163 µs | 428 |
| `RangeQuery` at revision | 3.5 µs | 28 |
| `DeleteRangeKeys` single key | 72 µs | 150 |
| `CurrentRevisionValue` | 0.6 µs | 10 |

## Storage — structured config CRUD

`internal/storage/bbolt/config/repository.go`

| Operation | Time | Allocations |
|---|---|---|
| `Create` | 86 µs | 184 |
| `Get` | 3.9 µs | 25 |
| `Update` | 61 µs | 157 |
| `Delete` | 73 µs | 142 |
| `ListSummariesByPrefix`, 1000 configs | 1.71 ms | 5029 |
| `ListSummaryPage` (20 of 1000) | 1.52 ms | 1105 |
| `SearchByPath` over 1000 configs | 140 µs | 2051 |
| `GetConfigHistory`, 20 of 50 revisions | 79 µs | 492 |

## etcd wire operations

`internal/handler/etcdv3/kv_server.go`, in-process (no gRPC listener)

| Operation | Time | Allocations |
|---|---|---|
| `Put` | 77 µs | 180 |
| `Put`, **durable** | 8.3 ms | 167 |
| `Txn` compare-and-swap | 84 µs | 213 |
| `Txn` compare-and-swap, contended | 83 µs | 214 |
| `Txn` compare fails (no write) | 3.7 µs | 41 |
| `Txn` read-only range | 5.3 µs | 60 |
| `Txn` 1 put | 78 µs | 185 |
| `Txn` 5 puts | 394 µs | 914 |
| `Txn` 20 puts | 2.13 ms | 3688 |
| `Txn` 1 put, **durable** | 8.1 ms | 172 |
| `Txn` 5 puts, **durable** | 41.9 ms | 859 |
| `Txn` 20 puts, **durable** | 169 ms | 3459 |

## Watch fan-out

`internal/transport/watch/publisher.go`

| Operation | Time | Allocations |
|---|---|---|
| Deliver to 1 matching subscriber | 156 ns | 0 |
| Deliver to 10 | 697 ns | 0 |
| Deliver to 100 | 5.6 µs | 0 |
| Deliver to 1000 | 90 µs | 0 |
| 1000 subscribers, none matching | 18 µs | 0 |
| 100 subscribers, buffers full (events dropped) | 8.4 µs | 299 |
| Subscribe + unsubscribe | 1.6 µs | 6 |

## Schema validation on the write path

`internal/service/schemavalidator/validator.go`

| Operation | Time | Allocations |
|---|---|---|
| Validate JSON, schema cached | 4.7 µs | 64 |
| Validate YAML, schema cached | 13.5 µs | 140 |
| Validate, schema not cached (compiles) | 86 µs | 748 |
| Validate, 10 attachments in namespace | 11.3 µs | 217 |
| Validate, 100 attachments in namespace | 76 µs | 1747 |
| Validate, payload rejected | 4.1 µs | 65 |
| Validator early-return, no schema matched | 8 ns | 0 |

The early-return figure excludes the storage lookup that precedes it — the
fake store in that benchmark answers from memory. It says only that once the
lookup comes back empty, the validator adds nothing.

## What these numbers say

**One `fsync` per write, and it is ~99% of the cost.** A durable write is
8.2 ms against 77 µs of actual work. Anything that changes how many
transactions a request opens changes client-visible latency by milliseconds,
not microseconds.

**`Txn` opens one transaction per operation today.** The 1/5/20-put curve is
linear — 8.1 ms, 41.9 ms, 169 ms durable — because each op commits separately.
Wrapping a whole `Txn` in one transaction should collapse that to roughly the
cost of a single commit. The widely-voiced concern that a wider transaction
serialises more applies to contention *between* concurrent `Txn` calls, not to
the multi-op case, which stands to get dramatically faster.

**Writes are already fully serialised.** Contended and sequential land at the
same ns/op, which is what a single-writer resource looks like under
`RunParallel`: throughput is capped at one writer regardless of how many are
waiting. Parity is the baseline here, not evidence that contention is free. If
the contended number ever rises above the sequential one, a transaction is
being held open longer per call.

**Paging saves allocations, not scanning.** Fetching 20 summaries out of 1000
costs 1.52 ms against 1.71 ms for fetching all of them — but 134 KB against
364 KB. The bbolt scan is the same; only materialisation is bounded.

**Schema path patterns are recompiled on every write.** The compiled *schema*
is cached; the glob in `findBestMatch` is not, and it runs once per attachment
per call. A namespace with 100 attachments therefore pays 76 µs and 1747
allocations on every single write, matching or not.

**Watch fan-out is allocation-free but linear in subscribers.** `notify` holds
a read lock and walks every subscription, so 1000 watchers cost 90 µs per
event even though only the matching ones are delivered to — 18 µs of that is
the walk and filter alone.
