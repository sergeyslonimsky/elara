# Usecase-owned transactions with context-injected tx handles

- Status: accepted
- Date: 2026-06-10
- Deciders: Elara maintainers
- Tags: storage, transactions, layering

Technical Story: EL-51 — Usecase-owned Transactions (`plans/EL-51/architecture.md`).
Landed across `cf364f0`, `8c107f1`, `2f6b415`, `71d1713` ("ref: update project
architecture", 2026-05-31 → 2026-06-10); the deactivate cascade that motivated it
shipped in `07b93ef` (#67); the dead `internal/service/storage` TxManager was
removed in `415c7b5` (#76).

## Context and Problem Statement

Elara is backed by a single [bbolt](https://github.com/etcd-io/bbolt) file. Before
EL-51, the *service* layer owned transactions through a `TxManager` whose `Tx`
type exposed `Bucket(name []byte)` — bbolt semantics leaking into a "backend
agnostic" API. Each service method opened its own transaction internally, e.g.
`sessions.RevokeAllForUser` wrapped its bulk-revoke in `txm.Write`.

This broke down the moment a usecase needed two service calls to be atomic. The
EL-50 user-deactivation flow must flip `User.Status` **and** revoke every session
in one unit of work. With each service opening its own transaction, that was
impossible: two independent transactions, no shared rollback. How should Elara
place transaction boundaries so multi-step writes are atomic, without threading a
`tx` object through every layer?

## Decision Drivers

- Multi-step writes spanning several repos/services must be atomic.
- Transaction boundaries should be visible where the unit of work is defined.
- Stop leaking bbolt (`Bucket`) into the storage contract; keep a Postgres swap open.
- Avoid duplicated method variants (`Update` vs `UpdateInTx`) and tx-mock boilerplate.

## Considered Options

1. **Explicit `tx` parameter** threaded through service and repo signatures.
2. **Service-owned transactions** (status quo) — each service opens its own.
3. **Usecase-owned transactions, tx handle injected via `context.Context`.**

## Decision Outcome

Chosen option: **3 — usecase-owned transactions with context injection.**

The only way to open a transaction is `storage.Manager.WithTx(ctx, func(ctx) error) error`
(`internal/storage/manager.go`). The **usecase** owns the boundary: it wraps the
whole unit of work in one `WithTx` call. `internal/usecase/user/service.go`
(`transitionStatus`) wraps `GetByID` + status change + `RevokeAllForUser` in a
single `WithTx`, so deactivation commits or rolls back as a whole.

Repos and services are **transaction-agnostic**. `WithTx` stores the `*bolt.Tx`
under a private context key (`pkg/bbolt/manager.go`, `txKey{}`); repos read it via
`GetQuerier(ctx)` — the active tx if one is in ctx, otherwise a short-lived
auto-tx. Signatures never mention `tx`; code in and out of a transaction is
identical. The reusable orchestration lives in `pkg/bbolt`, deliberately mirroring
`sergeyslonimsky/core/sql` so a Postgres adapter can be dropped in with no usecase
changes.

**Nested `WithTx` flattens.** The implementation checks for an existing writable
tx in ctx and, if present, just calls the callback in the same tx rather than
opening a new one:

```go
func (m *DBManager) WithTx(ctx context.Context, callback func(context.Context) error) error {
    if existing, ok := TxFromContext(ctx); ok && existing.Writable() {
        return callback(ctx) // reuse outer tx — no nested begin
    }
    tx, err := m.db.Begin(true)
    // ... runTx: commit on success, rollback on error/panic/Goexit
}
```

This is what makes service-level atomicity composable: a service can call `WithTx`
defensively, and when a usecase already opened one, it simply joins — bbolt allows
only one write-tx at a time, so flattening is the only safe behavior.

### Consequences

- Good: multi-step writes are atomic with the boundary declared in one place (the usecase).
- Good: the service layer shrank — no tx plumbing, no `txm.Write` wrappers; easier to test.
- Good: `Manager` is backend-agnostic; `pkg/bbolt` shadows `core/sql` for a future Postgres adapter.
- Good: single method per operation — no `*InTx` twins, no `tx` parameter threading.
- Bad: the tx handle travels **implicitly** via ctx. Coupling is invisible in signatures; a reader must know the convention.
- Bad: discipline required — a repo/service must never open a competing tx; it must join the ctx one. bbolt's single-writer model turns a mistake here into a deadlock or lost atomicity.
- Bad: auto-tx fallback means a bare repo call outside `WithTx` silently opens its own short-lived tx — convenient, but atomicity across such calls is not guaranteed.

## Pros and Cons of the Options

### Option 1 — Explicit `tx` parameter

- Good: coupling is visible; the type system tracks who is in a transaction.
- Good: no hidden ctx convention.
- Bad: every repo/service signature carries `tx`; adding/removing a boundary churns many APIs.
- Bad: tends toward `Update`/`UpdateInTx` duplication and leaks the tx type across layers.

### Option 2 — Service-owned transactions (previous state)

- Good: simple for single-service operations.
- Bad: cross-service atomicity is impossible — the EL-51 blocker.
- Bad: boundaries hidden inside services; `Tx.Bucket` leaked bbolt into the contract.

### Option 3 — Usecase-owned, context-injected (chosen)

- Good: composable atomicity, clean signatures, backend-agnostic contract.
- Good: nested `WithTx` flattens, so defensive wrapping is safe.
- Bad: implicit ctx coupling and a discipline requirement the compiler cannot enforce.

This is a genuine trade-off. The Go community is split on implicit ctx-based tx
propagation versus explicit parameters; Elara chose implicit for the leaner API,
accepting that correctness now rests on convention rather than the type system.
