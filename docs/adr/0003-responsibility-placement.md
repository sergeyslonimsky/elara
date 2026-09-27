# 3. Where a responsibility goes: the layer is decided by what the code depends on

- Status: accepted
- Date: 2026-09-27
- Deciders: Elara maintainers

## Context and Problem Statement

[ADR 0001](0001-usecase-owned-transactions.md) settled who owns a transaction.
[`docs/architecture.md`](../architecture.md) lists what each layer owns. Neither
answers the question that actually comes up while writing code: **given a new
responsibility, which layer does it go in?**

"No business logic in handlers" is a prohibition, not a test. It does not say
whether authorizing a caller is business logic, or who publishes a watch event,
or why a transaction boundary cannot sit in a handler. In practice those
questions get answered by looking at neighbouring code — which works only as
long as the neighbouring code is correct. `internal/handler/etcdv3` is not (see
Consequences), so reading it as a model reproduces the deviation.

This ADR states the rule that decides placement, and the test for applying it.

## Decision Drivers

- A new responsibility should have one obvious home, derivable without asking.
- The rule must survive a second transport: adding WebSocket or a CLI must not
  require moving business logic.
- Existing code may be in violation. The rule has to be usable as a standard
  that code is measured against, not as a description of what the code does.

## Considered Options

1. **Enumerate responsibilities per layer** — extend the layer table with every
   case as it comes up.
2. **Decide by dependency direction** — a responsibility belongs to the layer
   whose concerns it actually depends on.

## Decision Outcome

Chosen option: **2 — decide by dependency direction.**

**Handler** owns everything that depends on *how the call arrived*: the wire
format, decoding into internal structures, authentication, authorization of the
caller, and resolving who is calling. It hands the usecase a request plus a
caller identity — or no identity, where the usecase allows that, as a background
job has none. A handler never decides *what happens*.

**UseCase** owns everything that depends on *what the caller wants*: the flow,
the transaction boundary, the ordering of side effects against the commit, and
which services to call. It **receives** the caller's identity rather than
discovering it, and it publishes notifications.

**Service** does one concrete job — reach a repository, call an OIDC provider,
hash a password, validate against a schema, dispatch a webhook — and knows
nothing about the business case that invoked it.

### The test

Move the code to a different transport.

- If it must come along unchanged, it belongs in the **usecase**.
- If it only makes sense for the transport it is in, it belongs in the
  **handler**.

Applied to a watch notification: an etcd `Put` and a ConnectRPC `CreateConfig`
must both publish the event, and publishing does not change with the wire
format. Usecase. Applied to `checkAccess` on etcd token claims: a ConnectRPC
call authorizes through an interceptor over sessions instead, and the mechanism
is a property of the transport. Handler.

### Reference implementation

`internal/handler/v2/` follows this ADR: zero `Notify` calls, zero authorization
calls — authorization happens in `internal/handler/v2/interceptor`, and the
usecases publish their own events. When placement is unclear, diff the two
handler packages before designing anything; where they disagree, `v2` is right.

## Consequences

**Notification is a usecase concern.** `internal/transport/{watch,webhook}` holds
the publisher and the dispatcher because they are wire transports, but they are
*called* from the usecase. `internal/usecase/config/service_create.go`,
`service_update.go`, `service_delete.go` and `service_copy.go` are the model.

**The transaction boundary can only be usecase-owned** — this is ADR 0001's
conclusion re-derived from the rule rather than asserted. Side effects have to
be ordered against the commit: an event must not be published before the
transaction that produced it commits, and must not be published at all if it
rolls back. Deciding when to publish *is* deciding what happens, so a layer that
owns the boundary must be a layer allowed to make that decision. A handler is
not. It follows that a handler must never be handed a transaction boundary, not
even under another name.

**Authorization in a handler is not a violation of "no business logic".**
Permitting a caller is not deciding what happens to the data. The mechanism
differs per transport — ConnectRPC authorizes in an interceptor over session
identity, the etcd-compatible gRPC API authorizes in
`internal/handler/etcdv3/access.go` over service-token claims — and that
difference is exactly why it cannot be consolidated into a usecase.

**`pdp.Effective*` inside a usecase is scoping, not permission.**
`EffectiveNamespaces` and `EffectiveDomains` answer "which namespaces may this
principal see" and shape a result set; see
`internal/usecase/config/service_search.go` and
`internal/usecase/token/service_list.go`. That is business logic over an
identity the handler already resolved, and it does not make authorization a
usecase concern.

**The deviation that prompted this ADR is resolved.** When this was written,
`internal/handler/etcdv3/kv_server.go` orchestrated `Txn` — evaluating compares,
selecting the success/failure branch, looping operations — and published watch
events from `notifyPut` and from `DeleteRange`. Both moved into
`internal/usecase/config` (`service_txn.go`, `kv_events.go`) as part of making
`Txn` atomic, which could not be done correctly without the move: there is no way
to put a transaction boundary around orchestration that lives in a handler
without handing the handler that boundary.

That is the rule's first real test, and the outcome is worth recording. Paying
down the violation was not a tax on the feature — it *was* the feature. Twenty
writes in one transaction went from 165 ms to 9.4 ms once they shared a commit,
and contended compare-and-swap did not regress. See
[the performance baseline](../reference/performance.md).
