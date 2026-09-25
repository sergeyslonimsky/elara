package config_test

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	configrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/config"
	namespacerepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/namespace"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

// Benchmarks for the etcd-compatible raw KV path (etcd.go) — the surface the
// etcd-wire Put/Range/DeleteRange RPCs ultimately land on.
//
// These are the "before" numbers for making Txn atomic. That change moves a
// whole compare+ops sequence onto one outer bbolt transaction, and bbolt
// permits a single writer at a time, so strictly more work ends up
// serialised. Without a baseline the cost of that trade would be unknown
// rather than accepted.
//
// Every harness cost — key formatting, payload construction — is kept out of
// the timed section on purpose: b.ReportAllocs only says something useful if
// the allocations it counts belong to the code under test.
//
// Durability: these run without fsync unless the benchmark name ends in
// _Durable. bbolt fsyncs per commit, and that syscall costs ~8 ms here —
// enough to hide everything else, including the serialisation the atomic-Txn
// work changes. The mode is part of the name so benchstat cannot diff one
// against the other.

func BenchmarkPutKey_NewKey(b *testing.B) {
	b.ReportAllocs()

	repo, nsr := newBenchRepo(b)
	ctx := b.Context()
	seedNamespace(b, nsr, "ns")

	keys := benchKeys(b.N)
	value := benchPayload(256)

	b.ResetTimer()

	for i := range b.N {
		if _, _, err := repo.PutKey(ctx, "ns", keys[i], value); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPutKey_NewKey_Durable is the write latency a client actually
// observes: one fsync per key.
func BenchmarkPutKey_NewKey_Durable(b *testing.B) {
	b.ReportAllocs()

	repo, nsr := newBenchRepoDurable(b)
	ctx := b.Context()
	seedNamespace(b, nsr, "ns")

	keys := benchKeys(b.N)
	value := benchPayload(256)

	b.ResetTimer()

	for i := range b.N {
		if _, _, err := repo.PutKey(ctx, "ns", keys[i], value); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPutKey_Overwrite is the more expensive of the two Put paths: an
// existing key is read back as prevKV and a history entry is appended, so it
// touches strictly more buckets than a first write.
func BenchmarkPutKey_Overwrite(b *testing.B) {
	b.ReportAllocs()

	repo, nsr := newBenchRepo(b)
	ctx := b.Context()
	seedNamespace(b, nsr, "ns")

	const key = "/svc/hot.json"

	value := benchPayload(256)

	if _, _, err := repo.PutKey(ctx, "ns", key, value); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		if _, _, err := repo.PutKey(ctx, "ns", key, value); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPutKey_Contended is the number the atomic-Txn work is most likely
// to move: parallel writers on one key, where bbolt's single-writer
// transaction is the bottleneck rather than the work inside it.
func BenchmarkPutKey_Contended(b *testing.B) {
	b.ReportAllocs()

	repo, nsr := newBenchRepo(b)
	ctx := b.Context()
	seedNamespace(b, nsr, "ns")

	const key = "/svc/contended.json"

	value := benchPayload(256)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, _, err := repo.PutKey(ctx, "ns", key, value); err != nil {
				b.Error(err)

				return
			}
		}
	})
}

func BenchmarkRangeQuery_SingleKey(b *testing.B) {
	b.ReportAllocs()

	repo, nsr := newBenchRepo(b)
	ctx := b.Context()
	seedNamespace(b, nsr, "ns")

	const key = "/svc/single.json"

	if _, _, err := repo.PutKey(ctx, "ns", key, benchPayload(256)); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		if _, _, err := repo.RangeQuery(ctx, "ns", key, "", "", 0, 0, false); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRangeQuery_Prefix scales the matched-key count because the cost is
// per returned pair, not per call — a single number would hide which of the
// two dominates.
func BenchmarkRangeQuery_Prefix(b *testing.B) {
	for _, size := range []int{1, 10, 100, 1000} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			b.ReportAllocs()

			repo, nsr := newBenchRepo(b)
			ctx := b.Context()
			seedNamespace(b, nsr, "ns")

			seedKeys(b, repo, "ns", size)

			b.ResetTimer()

			for b.Loop() {
				// "/svc0" is "/svc/" with its last byte incremented — the
				// same exclusive prefix end etcd's clients compute.
				kvs, _, err := repo.RangeQuery(ctx, "ns", "/svc/", "ns", "/svc0", 0, 0, false)
				if err != nil {
					b.Fatal(err)
				}

				if len(kvs) != size {
					b.Fatalf("expected %d pairs, got %d", size, len(kvs))
				}
			}
		})
	}
}

// BenchmarkRangeQuery_KeysOnly pairs with the prefix benchmark above at the
// same size: the delta between them is what skipping value reads buys.
func BenchmarkRangeQuery_KeysOnly(b *testing.B) {
	b.ReportAllocs()

	repo, nsr := newBenchRepo(b)
	ctx := b.Context()
	seedNamespace(b, nsr, "ns")

	seedKeys(b, repo, "ns", 100)

	b.ResetTimer()

	for b.Loop() {
		if _, _, err := repo.RangeQuery(ctx, "ns", "/svc/", "ns", "/svc0", 0, 0, true); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRangeQuery_AtRevision reads through the history bucket instead of
// the current-value bucket — a different access pattern, not a variation in
// size, which is why it is measured separately.
func BenchmarkRangeQuery_AtRevision(b *testing.B) {
	b.ReportAllocs()

	repo, nsr := newBenchRepo(b)
	ctx := b.Context()
	seedNamespace(b, nsr, "ns")

	const key = "/svc/versioned.json"

	value := benchPayload(256)
	for range 10 {
		if _, _, err := repo.PutKey(ctx, "ns", key, value); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()

	for b.Loop() {
		if _, _, err := repo.RangeQuery(ctx, "ns", key, "", "", 0, 1, false); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDeleteRangeKeys_SingleKey(b *testing.B) {
	b.ReportAllocs()

	repo, nsr := newBenchRepo(b)
	ctx := b.Context()
	seedNamespace(b, nsr, "ns")

	keys := benchKeys(b.N)
	value := benchPayload(256)

	for i := range b.N {
		if _, _, err := repo.PutKey(ctx, "ns", keys[i], value); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()

	for i := range b.N {
		if _, _, err := repo.DeleteRangeKeys(ctx, "ns", keys[i], "", "", false); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCurrentRevisionValue(b *testing.B) {
	b.ReportAllocs()

	repo, nsr := newBenchRepo(b)
	ctx := b.Context()
	seedNamespace(b, nsr, "ns")

	if _, _, err := repo.PutKey(ctx, "ns", "/svc/x.json", benchPayload(16)); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		if _, err := repo.CurrentRevisionValue(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

// benchPayload returns a value of roughly config-like size. Callers build it
// before the timed section so its cost never lands in reported allocations.
func benchPayload(size int) []byte {
	v := make([]byte, size)
	for i := range v {
		v[i] = byte('a' + i%26)
	}

	return v
}

// benchKeys precomputes n distinct keys under the /svc/ prefix. Formatting a
// key allocates, so doing it inside a timed loop would show up as churn
// attributed to the repository.
func benchKeys(n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = "/svc/key-" + strconv.Itoa(i) + ".json"
	}

	return keys
}

func seedKeys(b *testing.B, repo *configrepo.Repository, namespace string, n int) {
	b.Helper()

	value := benchPayload(256)
	for _, k := range benchKeys(n) {
		if _, _, err := repo.PutKey(b.Context(), namespace, k, value); err != nil {
			b.Fatal(err)
		}
	}
}

// newBenchRepo opens a store that skips fsync on commit — the default for
// every benchmark here except the _Durable ones.
func newBenchRepo(b *testing.B) (*configrepo.Repository, *namespacerepo.Repository) {
	b.Helper()

	return newBenchRepoSync(b, true)
}

// newBenchRepoDurable opens a store with bbolt's real fsync-per-commit
// behaviour.
func newBenchRepoDurable(b *testing.B) (*configrepo.Repository, *namespacerepo.Repository) {
	b.Helper()

	return newBenchRepoSync(b, false)
}

// newBenchRepoSync mirrors newRepo from repository_test.go but owns its store
// so it can set the durability mode.
//
// It is a separate helper rather than a parameter on newRepo because newRepo
// is shared with the tests, and the tests have no business caring about fsync:
// threading a mode through them would be churn in the one place that should
// stay boring.
func newBenchRepoSync(b *testing.B, noSync bool) (*configrepo.Repository, *namespacerepo.Repository) {
	b.Helper()

	store, err := bbolt.Open(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}

	b.Cleanup(func() { _ = store.Close() })

	store.DB().NoSync = noSync

	mgr := pkgbbolt.NewManager(store.DB())

	return configrepo.NewRepository(mgr), namespacerepo.NewRepository(mgr)
}
