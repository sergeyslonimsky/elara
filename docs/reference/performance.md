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
| `Put` | 74 µs | 183 |
| `Put`, **durable** | 8.1 ms | 161 |
| `Txn` compare-and-swap | 77 µs | 210 |
| `Txn` compare-and-swap, contended | 81 µs | 215 |
| `Txn` compare fails (no write) | 3.9 µs | 42 |
| `Txn` read-only range | 4.2 µs | 53 |
| `Txn` 1 put | 76 µs | 194 |
| `Txn` 5 puts | 202 µs | 460 |
| `Txn` 20 puts | 570 µs | 1412 |
| `Txn` 1 put, **durable** | 8.2 ms | 177 |
| `Txn` 5 puts, **durable** | 8.5 ms | 422 |
| `Txn` 20 puts, **durable** | 9.4 ms | 1339 |

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
| Validate JSON, schema cached | 3.7 µs | 47 |
| Validate YAML, schema cached | 12.8 µs | 123 |
| Validate, schema not cached (compiles) | 87 µs | 731 |
| Validate, 1 attachment in namespace | 3.8 µs | 47 |
| Validate, 10 attachments in namespace | 4.2 µs | 47 |
| Validate, 100 attachments in namespace | 8.6 µs | 47 |
| Validate, payload rejected | 3.4 µs | 48 |
| Validator early-return, no schema matched | 8 ns | 0 |

The early-return figure excludes the storage lookup that precedes it — the
fake store in that benchmark answers from memory. It says only that once the
lookup comes back empty, the validator adds nothing.

## What these numbers say

**One `fsync` per write, and it is ~99% of the cost.** A durable write is
8.2 ms against 77 µs of actual work. Anything that changes how many
transactions a request opens changes client-visible latency by milliseconds,
not microseconds.

**A `Txn` is one transaction, and that is what makes multi-op cheap.** The
durable 1/5/20-put curve is 8.2 ms, 8.5 ms, 9.4 ms — almost flat, because all
the operations share a single commit and therefore a single `fsync`. Before
`Txn` became atomic each operation committed separately and the same curve read
8.1 ms, 40.4 ms, 165 ms. Twenty writes in one transaction went from 165 ms to
9.4 ms, a 17-fold improvement, with allocations down 54 %.

That is worth stating plainly because the expectation was the opposite: widening
a transaction was assumed to cost throughput, since bbolt permits one writer at
a time. It did not. The serialisation concern was real but it applies *between*
concurrent `Txn` calls, not to the operations inside one — and even there
nothing measurable appeared, with contended compare-and-swap coming out 3 %
*faster* than before.

**Writes are fully serialised, and contended matches sequential.** Under
`RunParallel` a single-writer resource lands at the same ns/op as its sequential
benchmark, because throughput is capped at one writer no matter how many wait.
Parity is therefore the baseline, not evidence that contention is free. If the
contended number ever rises above the sequential one, a transaction is being
held open longer per call.

**Paging saves allocations, not scanning.** Fetching 20 summaries out of 1000
costs 1.52 ms against 1.71 ms for fetching all of them — but 134 KB against
364 KB. The bbolt scan is the same; only materialisation is bounded.

**Schema validation no longer scales with how much is configured.** Allocations
per validated write are flat at 47 whether the namespace holds 1 attachment or
100, and time grows only with the glob matching itself: 3.8 µs → 8.6 µs across
that range.

This is the first thing the benchmarks caught. The compiled *schema* was
cached, but the glob in `findBestMatch` was not, and it ran once per attachment
on every write: a namespace with 100 attachments paid 86 µs and 1846
allocations per write, matching or not. Caching the compiled pattern alongside
its specificity score cut that by 90% in time and 97% in allocations, and made
the allocation count independent of configuration size.

**Watch fan-out is allocation-free but linear in subscribers.** `notify` holds
a read lock and walks every subscription, so 1000 watchers cost 90 µs per
event even though only the matching ones are delivered to — 18 µs of that is
the walk and filter alone.
