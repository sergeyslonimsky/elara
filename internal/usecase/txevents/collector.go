// Package txevents defers notifications until the transaction that produced them
// has committed.
//
// An event must never be published before its transaction commits — a watcher
// would observe a write that a later failure rolled back, or, worse, read the
// store immediately after the notification and still see the old state, then
// wait forever for a second notification that never comes. It must also not be
// published at all if the transaction fails. Deferring is therefore not an
// optimisation; it is what makes notification consistent with the store.
//
// The collector rides in the context for the same reason the transaction handle
// does (see docs/adr/0001-usecase-owned-transactions.md): the operations that
// produce events must not care whether they run standalone or nested inside a
// larger flow. Whoever opens the outermost transaction installs the collector and
// flushes it after the commit returns; a nested call finds one installed and only
// appends.
//
// It lives in its own package rather than inside a single usecase because the
// outermost transaction is not always opened by the usecase that writes: revoking
// a lease spans the lease repository and the config write path, and the lease
// usecase is the one that owns the boundary.
package txevents

import "context"

// Collector buffers notifications for one transaction.
//
// Not guarded by a mutex, deliberately. A collector belongs to a single
// transaction, and the storage backend permits one writer, so the operations
// appending to it run sequentially. Fanning out inside a transaction would break
// more than this.
type Collector struct {
	notify []func(context.Context)
}

type collectorKey struct{}

// Install returns a ctx carrying a collector, the collector itself, and whether
// this caller is the one that installed it.
//
// Only the installer may flush. That is what makes an operation behave the same
// way standalone and nested: called on its own, a write installs the collector
// and publishes after its own commit; called from inside a larger transaction, it
// finds the outer collector and leaves publication to the boundary that owns it.
func Install(ctx context.Context) (context.Context, *Collector, bool) {
	if existing, ok := ctx.Value(collectorKey{}).(*Collector); ok {
		return ctx, existing, false
	}

	c := &Collector{}

	return context.WithValue(ctx, collectorKey{}, c), c, true
}

// Add buffers a notification.
func (c *Collector) Add(fn func(context.Context)) {
	c.notify = append(c.notify, fn)
}

// Flush publishes everything buffered, in the order it was produced.
//
// The ctx passed here must NOT be the one from inside the transaction: that one
// carries a handle to a transaction that is already over, and a repository call
// made with it would join something finished. Callers pass the ctx they held
// before opening the boundary.
func (c *Collector) Flush(ctx context.Context) {
	for _, fn := range c.notify {
		fn(ctx)
	}

	c.notify = nil
}
