# Lease expiry decided from stored state, with throttled renewals

- Status: accepted
- Date: 2026-09-28
- Deciders: Elara maintainers
- Tags: etcd-wire, leases, storage, replication

Technical Story: the Lease API work that made etcd's `clientv3/concurrency`
recipes usable against Elara. Landed across `1042093` (domain), `9d1355e`
(repository), `bef999b` (KV write path), `880fd3b` (usecase), `48fc38e` (server),
`f02d684` (expiry sweep) and `969fb28` (acceptance suite).

## Context and Problem Statement

A lease is a TTL grant that keys attach to: when it expires, every key attached
to it is deleted. It is the mechanism behind distributed locks and leader
election — `concurrency.Session` holds a lease, the lock key is written under it,
and *losing the lease is what releases the lock*. That makes expiry a correctness
property, not a cleanup nicety: a lease that outlives its TTL is a lock nobody can
take, and a lease that dies early is a lock two holders think they own.

Two questions had to be answered before writing any of it:

1. **Where does the expiry instant live, and who decides that a lease has
   expired?** The obvious implementation — etcd's own — keeps a min-heap of
   expiry times in the process and fires a timer.
2. **How often does a renewal reach the store?** `KeepAlive` arrives roughly
   every TTL/3 *per session*, so this decides what a fleet of sessions costs.

The second question has an easy answer if the first one is "in memory". Both
become harder because of a constraint that is not in the code: the README's
roadmap commits to Raft-based HA after 1.0. Elara is single-instance **today**,
because bbolt holds an exclusive file lock — not by design. Anything built on the
assumption "there is one process, and it remembers things" is work that has to be
thrown away rather than extended.

## Decision Drivers

- Expiry must survive a process restart without a warm-up or a rebuild step.
- The design must still be correct when the store replicates: a decision made
  from one node's memory is not a decision the other nodes can reach.
- bbolt permits a single writer, so a per-renewal write competes with every other
  write in the service.
- An operation that can be retried is worth more than one that must not be, since
  a sweep will inevitably race clients, restarts and locked namespaces.

## Considered Options

For **where expiry lives**:

1. **In-memory lessor**, as etcd does: a heap of expiry times, rebuilt from the
   store at boot, with timers firing revocations.
2. **Expiry in the record**, with a background sweep asking the store what is due.
3. **Hybrid**: the heap drives timing, the store is a durability journal.

For **how often renewals are persisted**:

- A. Persist every renewal.
- B. Persist only once the renewal has drifted past a fraction of the TTL.
- C. etcd's lease checkpointing: keep remaining TTL in memory, persist
  periodically through a dedicated path.

## Decision Outcome

Chosen: **2 — expiry in the record**, with **B — throttled persistence**.

### Expiry is a property of the record, not of a timer

`domain.Lease` carries `ExpiresAt`, and it is written to bbolt along with two
indexes: `lease_keys` (which keys a lease holds) and `lease_expiry`, keyed by
`<big-endian expiry><big-endian id>` so "what is due" is a prefix walk that stops
at the first lease that is not.

The worker (`internal/usecase/lease/expirer.go`) decides only *when to look*.
What has expired is decided inside the transaction, from stored state and an
instant passed in as an argument:

```go
func (s *Service) RevokeExpired(ctx context.Context, now time.Time, limit int) (int, error)
```

`now` is a parameter rather than a call to `time.Now` inside, and that is the
whole point. The same call applied on another node reaches the same conclusion
instead of depending on when that node happened to tick — which is what a
replicated apply requires. It also makes the operation replayable and its tests
free of a fake clock.

**Each lease is revoked in its own transaction.** One lease that cannot be
revoked right now — an operator locked its namespace — must not roll back the
ones already done, nor stop the loop. Its error is reported alongside the count;
the next sweep retries it, which is safe because revoking is idempotent.

### Renewals are answered from the entity and persisted on drift

`KeepAlive` returns a full TTL from now and writes the new expiry only when
`domain.Lease.NeedsCheckpoint` says the drift has earned it — by default half a
TTL (`lease.checkpoint.threshold`).

The threshold is clamped to `(0, 1]`, and one TTL of drift is a hard ceiling, not
a stylistic bound: the promised expiry is a full TTL ahead of the renewal instant,
so a drift of exactly TTL means the *stored* expiry has just reached the present.
Tolerating more would let the sweep — which reads the store, not the promise —
revoke a lease its client is renewing on schedule.

Measured on an M1, `-count=6` (see `docs/reference/performance.md`):

| | |
|---|---|
| `KeepAlive`, throttled | 1.8 µs |
| `KeepAlive`, persisting every renewal | 22 µs |
| `KeepAlive`, persisting every renewal, durable | 8.1 ms |

Twelve times against the code path; four thousand against a store that fsyncs.
Once the store replicates, the same ratio is Raft-log volume.

### Notifications are published by whoever owns the transaction

Revoking a lease deletes its keys through the config write path, which defers
watch notifications until its transaction commits. Nested inside the lease
usecase's transaction, that deferral flushed too early — after its own flattened
call returned, before the outer commit. A watcher was told of a delete it could
not yet read, and waited forever for a second event that had already been sent.
That is how a waiting lock or election hangs.

The collector therefore lives in `internal/usecase/txevents`, outside both
usecases, and is installed by whoever opens the outermost transaction. The
transaction owner is not always the writer, and the notification boundary has to
follow the transaction, not the layer.

### Consequences

- Good: a restart loses nothing and needs no warm-up — the sweep reads what is
  due from the store on its next tick.
- Good: nothing about the design assumes a single process, so HA extends it
  rather than replacing it.
- Good: a missed or delayed tick is harmless; the next one picks up everything
  that came due, bounded by the batch size.
- Good: the common `KeepAlive` costs no transaction at all.
- Bad: **bounded staleness.** After an abrupt process death a lease can expire up
  to `threshold × TTL` earlier than the client was last promised. This is the
  same trade etcd makes with lease checkpointing off, and it is documented in
  `docs/reference/etcd-compatibility.md` rather than left in a code comment.
- Bad: expiry is granular to the sweep interval (one second by default), so a key
  can outlive its TTL by that much.
- Bad: the sweep pays a short index scan every tick even when nothing is due.
- Bad: correctness now depends on the `lease_expiry` index being maintained on
  every write — in particular, a renewal must delete the stale index entry before
  writing the new one, since the key encodes the expiry. There is a test for
  exactly that, because nothing else would catch it: the lease record stays
  correct while the index rots.

## Pros and Cons of the Options

### Option 1 — In-memory lessor (etcd's design)

- Good: precise timing without polling; no periodic scan.
- Good: proven at scale by etcd itself.
- Bad: the truth about when a lease dies lives in one process's memory. With
  replication, the node holding it stops being the place where that truth is, and
  the design has to be rebuilt around checkpointing rather than extended.
- Bad: two sources of state to keep in step, and the divergence between them is
  its own class of bug.

### Option 2 — Expiry in the record, swept from the store (chosen)

- Good: one source of truth; restart- and replication-safe by construction.
- Good: the decision is made inside a transaction, so it is idempotent and
  replayable.
- Bad: resolution is bounded by the sweep interval; a scan runs even when idle.

### Option 3 — Hybrid heap plus journal

- Good: the precision of option 1 with the durability of option 2.
- Bad: the most code of the three, and it takes on the divergence problem of
  option 1 long before a second node exists to justify it.

### Persisting every renewal (option A)

- Good: the stored expiry is always exact; nothing is lost on a crash.
- Bad: a write transaction per ping per session, against a single-writer store,
  and later a Raft entry per ping. Measured 12× (and 4400× durable) more
  expensive for an exactness whose absence costs bounded staleness.

### etcd-style checkpointing (option C)

- Good: the full etcd semantics, including TTL surviving a leader change.
- Bad: it answers a question Elara does not have yet — there is no leader change
  to survive — while adding a second source of truth today.

Revisit this ADR when Raft lands: option C stops being premature at the moment a
second node exists, and the threshold clamp argued above is exactly the place
where leader-change semantics will need to be reconsidered.
