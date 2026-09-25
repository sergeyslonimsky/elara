package etcdv3_test

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"go.etcd.io/etcd/api/v3/etcdserverpb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/etcdv3"
	"github.com/sergeyslonimsky/elara/internal/service/schemavalidator"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	configrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/config"
	namespacerepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/namespace"
	schemarepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/schema"
	configuc "github.com/sergeyslonimsky/elara/internal/usecase/config"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

// Benchmarks for Txn — the baseline that has to exist before Txn is made
// atomic.
//
// Today every compare and every op inside a Txn runs in its own usecase-level
// transaction. Making Txn atomic wraps the whole compare+ops sequence in one
// outer transaction, and bbolt permits a single writer at a time, so the
// change necessarily serialises more. The numbers below are what "more" gets
// compared against; without them the trade would be assumed rather than
// measured.
//
// These run against a real bbolt store and the real usecase, in-process, with
// no gRPC listener: the wire layer would add its own latency and variance to
// a number that is meant to be about storage. Unlike kv_server_test.go, which
// uses a fake repo, nothing here would show the locking profile at all if the
// store were faked.
//
// Durability: every benchmark here runs without fsync unless its name ends in
// _Durable. bbolt fsyncs on every commit, and that single syscall costs ~8 ms
// on this codebase — two to three orders of magnitude more than the work
// inside the transaction, which would bury the locking-profile change the
// atomic-Txn work actually makes and swamp benchstat with disk variance on
// shared CI runners. The _Durable benchmarks keep the real production latency
// on the record. The mode is part of the benchmark name on purpose: benchstat
// keys on names, so it cannot silently diff one mode against the other.

func BenchmarkPut(b *testing.B) {
	b.ReportAllocs()

	srv, ctx := newBenchKVServer(b)
	req := &etcdserverpb.PutRequest{
		Key:   []byte(benchLockKey),
		Value: benchTxnValue,
	}

	b.ResetTimer()

	for b.Loop() {
		if _, err := srv.Put(ctx, req); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPut_Durable is BenchmarkPut with bbolt's real fsync-per-commit
// behaviour — the write latency a client actually observes.
func BenchmarkPut_Durable(b *testing.B) {
	b.ReportAllocs()

	srv, ctx := newBenchKVServerDurable(b)
	req := &etcdserverpb.PutRequest{
		Key:   []byte(benchLockKey),
		Value: benchTxnValue,
	}

	b.ResetTimer()

	for b.Loop() {
		if _, err := srv.Put(ctx, req); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkTxn_CompareAndSwap is the steady-state lock-renewal shape: compare
// the held value, then write it back. One compare read plus one write per
// iteration, which after the atomic-Txn change becomes one transaction
// instead of two.
func BenchmarkTxn_CompareAndSwap(b *testing.B) {
	b.ReportAllocs()

	srv, ctx := newBenchKVServer(b)
	seedBenchKey(b, srv, ctx)

	req := benchCASRequest()

	b.ResetTimer()

	for b.Loop() {
		resp, err := srv.Txn(ctx, req)
		if err != nil {
			b.Fatal(err)
		}

		if !resp.GetSucceeded() {
			b.Fatal("compare unexpectedly failed on an uncontended key")
		}
	}
}

// BenchmarkTxn_CompareAndSwap_Contended is the number most likely to move
// when Txn becomes atomic: parallel writers racing on one key, where bbolt's
// single-writer transaction is the bottleneck rather than the work inside it.
//
// Both branches carry the same Put, so an iteration performs exactly one
// write whether its compare won or lost. Without that, rising contention
// would quietly turn write iterations into read-only ones and the benchmark
// would report a speed-up as throughput got worse.
//
// How to read it: RunParallel reports wall time divided by total iterations,
// so a perfectly serialised resource lands at the same ns/op as the
// sequential benchmark — which is where this sits today. Parity is therefore
// the baseline, not a sign that contention is free. If this number rises
// above BenchmarkTxn_CompareAndSwap after the atomic-Txn change, the widened
// transaction is holding the writer lock for longer per Txn.
func BenchmarkTxn_CompareAndSwap_Contended(b *testing.B) {
	b.ReportAllocs()

	srv, ctx := newBenchKVServer(b)
	seedBenchKey(b, srv, ctx)

	req := benchCASRequest()
	req.Failure = req.GetSuccess()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := srv.Txn(ctx, req); err != nil {
				b.Error(err)

				return
			}
		}
	})
}

// BenchmarkTxn_CompareFails isolates the read half: the compare cannot match,
// and the failure branch is empty, so no write happens. The delta against
// BenchmarkTxn_CompareAndSwap is what the write costs.
func BenchmarkTxn_CompareFails(b *testing.B) {
	b.ReportAllocs()

	srv, ctx := newBenchKVServer(b)
	seedBenchKey(b, srv, ctx)

	req := &etcdserverpb.TxnRequest{
		Compare: []*etcdserverpb.Compare{{
			Key:         []byte(benchLockKey),
			Result:      etcdserverpb.Compare_EQUAL,
			Target:      etcdserverpb.Compare_VALUE,
			TargetUnion: &etcdserverpb.Compare_Value{Value: []byte("never-written")},
		}},
	}

	b.ResetTimer()

	for b.Loop() {
		resp, err := srv.Txn(ctx, req)
		if err != nil {
			b.Fatal(err)
		}

		if resp.GetSucceeded() {
			b.Fatal("compare unexpectedly matched")
		}
	}
}

// BenchmarkTxn_MultiOp scales the number of writes inside one Txn. This is
// the shape the atomic-Txn change alters most: today n ops cost n
// transactions, afterwards they share one, so the slope of this curve is the
// thing to re-measure.
func BenchmarkTxn_MultiOp(b *testing.B) {
	benchTxnMultiOp(b, newBenchKVServer)
}

// BenchmarkTxn_MultiOp_Durable is the same curve with fsync on. It is the most
// consequential production number in this file: one fsync per op today means
// the slope is a real per-op disk cost, not just bookkeeping.
func BenchmarkTxn_MultiOp_Durable(b *testing.B) {
	benchTxnMultiOp(b, newBenchKVServerDurable)
}

// BenchmarkTxn_RangeOnly covers the read-only Txn that clients use to fetch
// several keys in one round trip — the one Txn shape the atomic change should
// leave alone, so a regression here would mean reads got dragged onto the
// write transaction.
func BenchmarkTxn_RangeOnly(b *testing.B) {
	b.ReportAllocs()

	srv, ctx := newBenchKVServer(b)
	seedBenchKey(b, srv, ctx)

	req := &etcdserverpb.TxnRequest{
		Success: []*etcdserverpb.RequestOp{{
			Request: &etcdserverpb.RequestOp_RequestRange{
				RequestRange: &etcdserverpb.RangeRequest{Key: []byte(benchLockKey)},
			},
		}},
	}

	b.ResetTimer()

	for b.Loop() {
		if _, err := srv.Txn(ctx, req); err != nil {
			b.Fatal(err)
		}
	}
}

const (
	// benchLockKey is "/bench/lock.json" split by the handler into namespace
	// "bench" and path "/lock.json". The namespace auto-vivifies on the first
	// Put, so nothing needs seeding up front.
	benchLockKey = "/bench/lock.json"

	benchHeldValue = "held"
)

var benchTxnValue = []byte(benchHeldValue)

// benchTxnMultiOp runs the multi-op curve against whichever store the caller
// picks, so the NoSync and fsync variants cannot drift apart.
func benchTxnMultiOp(
	b *testing.B,
	newServer func(*testing.B) (*etcdv3.KVServer, context.Context),
) {
	b.Helper()

	for _, ops := range []int{1, 5, 20} {
		b.Run(strconv.Itoa(ops), func(b *testing.B) {
			b.ReportAllocs()

			srv, ctx := newServer(b)

			req := &etcdserverpb.TxnRequest{Success: make([]*etcdserverpb.RequestOp, 0, ops)}
			for i := range ops {
				req.Success = append(req.Success, &etcdserverpb.RequestOp{
					Request: &etcdserverpb.RequestOp_RequestPut{
						RequestPut: &etcdserverpb.PutRequest{
							Key:   []byte("/bench/multi-" + strconv.Itoa(i) + ".json"),
							Value: benchTxnValue,
						},
					},
				})
			}

			b.ResetTimer()

			for b.Loop() {
				if _, err := srv.Txn(ctx, req); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// newBenchKVServer wires a KVServer onto a store that skips fsync on commit —
// the default for everything here except the _Durable benchmarks.
func newBenchKVServer(b *testing.B) (*etcdv3.KVServer, context.Context) {
	b.Helper()

	return newBenchKVServerSync(b, true)
}

// newBenchKVServerDurable wires a KVServer onto a store with bbolt's real
// fsync-per-commit behaviour.
func newBenchKVServerDurable(b *testing.B) (*etcdv3.KVServer, context.Context) {
	b.Helper()

	return newBenchKVServerSync(b, false)
}

// newBenchKVServerSync wires a KVServer onto a real bbolt store and the real
// usecase.
//
// This is a test file reaching across layers on purpose: the whole point is to
// measure the storage transaction the handler ultimately opens, which a
// handler-only unit harness cannot see.
//
// noSync is set through bbolt's public DB field rather than through
// bbolt.Open, which hardcodes its options — no production code has to grow a
// knob it would never use outside a benchmark.
func newBenchKVServerSync(b *testing.B, noSync bool) (*etcdv3.KVServer, context.Context) {
	b.Helper()

	store, err := bbolt.Open(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}

	b.Cleanup(func() { _ = store.Close() })

	store.DB().NoSync = noSync

	pkgMgr := pkgbbolt.NewManager(store.DB())
	configs := configrepo.NewRepository(pkgMgr)
	pub := benchWatcher{}

	// pdp is nil deliberately. The etcd-compatible KV path authorises in the
	// handler through authctx.Claims (access.go); service_kv.go never
	// consults the policy decision point. A permissive stub would silently
	// absorb it if that ever changed — nil makes the benchmark panic instead
	// of quietly measuring a different code path.
	usecase := configuc.New(
		bbolt.NewManager(store.DB()),
		nil,
		configs,
		pub,
		namespacerepo.NewRepository(pkgMgr),
		schemavalidator.New(schemarepo.NewRepository(pkgMgr)),
	)

	// A context carrying no claims is what namespaceAllowed treats as "auth
	// disabled, allowed" — the interceptor is not part of what is measured.
	return etcdv3.NewKVServer(usecase, pub), b.Context()
}

// benchCASRequest builds the compare-and-swap request once. Txn does not
// mutate its request, so the same message is reused across iterations rather
// than rebuilt inside the timed loop.
func benchCASRequest() *etcdserverpb.TxnRequest {
	return &etcdserverpb.TxnRequest{
		Compare: []*etcdserverpb.Compare{{
			Key:         []byte(benchLockKey),
			Result:      etcdserverpb.Compare_EQUAL,
			Target:      etcdserverpb.Compare_VALUE,
			TargetUnion: &etcdserverpb.Compare_Value{Value: benchTxnValue},
		}},
		Success: []*etcdserverpb.RequestOp{{
			Request: &etcdserverpb.RequestOp_RequestPut{
				RequestPut: &etcdserverpb.PutRequest{
					Key:   []byte(benchLockKey),
					Value: benchTxnValue,
				},
			},
		}},
	}
}

func seedBenchKey(b *testing.B, srv *etcdv3.KVServer, ctx context.Context) {
	b.Helper()

	_, err := srv.Put(ctx, &etcdserverpb.PutRequest{
		Key:   []byte(benchLockKey),
		Value: benchTxnValue,
	})
	if err != nil {
		b.Fatal(err)
	}
}

// benchWatcher is a no-op stand-in for the watch publisher, serving both the
// usecase's watcher seam and the handler's KVPublisher.
//
// Two reasons it is not the real thing. Layering: depguard forbids
// internal/handler from importing internal/transport, and a test file is no
// exception. Measurement: fan-out cost belongs to the publisher and is
// benchmarked in its own package, so keeping it out of here leaves these
// numbers about the storage transaction — which is what the atomic-Txn work
// changes.
type benchWatcher struct{}

func (benchWatcher) NotifyCreated(context.Context, *domain.Config) {}

func (benchWatcher) NotifyUpdated(context.Context, *domain.Config) {}

func (benchWatcher) NotifyDeleted(context.Context, string, string, int64) {}

func (benchWatcher) NotifyConfigLocked(context.Context, *domain.Config) {}

func (benchWatcher) NotifyConfigUnlocked(context.Context, *domain.Config) {}

func (benchWatcher) Subscribe(
	context.Context,
	string, string,
) (<-chan domain.WatchEvent, func()) {
	return nil, func() {}
}
