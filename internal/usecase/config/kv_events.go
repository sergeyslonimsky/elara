package config

import "context"

// pendingEvents buffers watch notifications until the transaction that produced
// them commits.
//
// It rides in the context for the same reason the transaction handle does (see
// docs/adr/0001-usecase-owned-transactions.md): the methods that produce events
// must not care whether they are running standalone or nested inside a Txn.
// Whichever call opens the outermost transaction installs the collector and
// flushes it after the commit returns; a nested call finds it already installed
// and only appends.
//
// An event must never be published before its transaction commits — a watcher
// would observe a write that a later failed operation rolled back — and must not
// be published at all if it does not. Deferring is therefore not an
// optimisation; it is what makes the notification consistent with the store.
//
// Not guarded by a mutex, deliberately. A collector belongs to one transaction,
// and bbolt permits a single writer, so the operations appending to it run
// sequentially. Fanning out inside a transaction would break more than this.
type pendingEvents struct {
	notify []func(context.Context)
}

type pendingEventsKey struct{}

func (p *pendingEvents) add(fn func(context.Context)) {
	p.notify = append(p.notify, fn)
}

// flush publishes everything buffered, in the order it was produced.
//
// The ctx passed here must NOT be the one from inside WithTx: that one carries a
// committed transaction handle, and a repository call made with it would join a
// transaction that is already over. Callers pass the ctx they held before
// opening the boundary.
func (p *pendingEvents) flush(ctx context.Context) {
	for _, fn := range p.notify {
		fn(ctx)
	}

	p.notify = nil
}

// withPendingEvents returns a ctx carrying a collector, plus the collector and
// whether this caller installed it.
//
// Only the installer flushes. That is what makes an operation behave the same
// way standalone and nested: called on its own, a write installs the collector
// and publishes after its own commit; called from inside Txn, it finds Txn's
// collector and leaves publication to the outer boundary.
func withPendingEvents(ctx context.Context) (context.Context, *pendingEvents, bool) {
	if existing, ok := ctx.Value(pendingEventsKey{}).(*pendingEvents); ok {
		return ctx, existing, false
	}

	p := &pendingEvents{}

	return context.WithValue(ctx, pendingEventsKey{}, p), p, true
}
