package config_test

import (
	"strconv"
	"testing"

	configrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/config"
)

// Benchmarks for the structured config-CRUD path (repository.go) — what the
// Web UI and the ConnectRPC API write through, as opposed to the raw KV path
// in etcd_bench_test.go.
//
// Worth measuring separately from the KV path because a write here fans out
// into more buckets: the config entry, a history entry and a changelog entry
// all land in one transaction, so this is where the write amplification of a
// widened transaction would show up first.

func BenchmarkRepository_Create(b *testing.B) {
	b.ReportAllocs()

	repo, nsr := newBenchRepo(b)
	ctx := b.Context()
	seedNamespace(b, nsr, "ns")

	paths := benchKeys(b.N)
	content := string(benchPayload(256))

	b.ResetTimer()

	for i := range b.N {
		if err := repo.Create(ctx, newTestConfig("ns", paths[i], content)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRepository_Get(b *testing.B) {
	b.ReportAllocs()

	repo, nsr := newBenchRepo(b)
	ctx := b.Context()
	seedNamespace(b, nsr, "ns")

	const path = "/svc/get.json"

	if err := repo.Create(ctx, newTestConfig("ns", path, string(benchPayload(256)))); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		if _, err := repo.Get(ctx, path, "ns"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRepository_Update grows the history bucket by one entry per
// iteration, which is the point: the cost of a write is not constant over the
// lifetime of a key, and this is the path where that shows.
func BenchmarkRepository_Update(b *testing.B) {
	b.ReportAllocs()

	repo, nsr := newBenchRepo(b)
	ctx := b.Context()
	seedNamespace(b, nsr, "ns")

	const path = "/svc/update.json"

	if err := repo.Create(ctx, newTestConfig("ns", path, "v0")); err != nil {
		b.Fatal(err)
	}

	cfg, err := repo.Get(ctx, path, "ns")
	if err != nil {
		b.Fatal(err)
	}

	content := string(benchPayload(256))

	b.ResetTimer()

	for b.Loop() {
		cfg.Content = content
		if err := repo.Update(ctx, cfg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRepository_Delete(b *testing.B) {
	b.ReportAllocs()

	repo, nsr := newBenchRepo(b)
	ctx := b.Context()
	seedNamespace(b, nsr, "ns")

	paths := seedConfigs(b, repo, "ns", b.N)

	b.ResetTimer()

	for i := range b.N {
		if _, err := repo.Delete(ctx, paths[i], "ns"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRepository_ListSummariesByPrefix is the unbounded listing the UI
// used before paging existed; it is kept as the worst case to compare the
// paged call against.
func BenchmarkRepository_ListSummariesByPrefix(b *testing.B) {
	for _, size := range []int{10, 100, 1000} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			b.ReportAllocs()

			repo, nsr := newBenchRepo(b)
			ctx := b.Context()
			seedNamespace(b, nsr, "ns")

			seedConfigs(b, repo, "ns", size)

			b.ResetTimer()

			for b.Loop() {
				got, err := repo.ListSummariesByPrefix(ctx, "/svc/", "ns")
				if err != nil {
					b.Fatal(err)
				}

				if len(got) != size {
					b.Fatalf("expected %d summaries, got %d", size, len(got))
				}
			}
		})
	}
}

// BenchmarkRepository_ListSummaryPage holds the page size fixed while the
// underlying set grows. A paged listing that still costs O(total) per call
// would be invisible in a single-size benchmark.
func BenchmarkRepository_ListSummaryPage(b *testing.B) {
	for _, size := range []int{10, 100, 1000} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			b.ReportAllocs()

			repo, nsr := newBenchRepo(b)
			ctx := b.Context()
			seedNamespace(b, nsr, "ns")

			seedConfigs(b, repo, "ns", size)

			b.ResetTimer()

			for b.Loop() {
				if _, _, err := repo.ListSummaryPage(ctx, "/svc/", "ns", 20, 0); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkRepository_SearchByPath(b *testing.B) {
	b.ReportAllocs()

	repo, nsr := newBenchRepo(b)
	ctx := b.Context()
	seedNamespace(b, nsr, "ns")

	seedConfigs(b, repo, "ns", 1000)

	b.ResetTimer()

	for b.Loop() {
		if _, err := repo.SearchByPath(ctx, "key-42", "ns"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRepository_GetConfigHistory(b *testing.B) {
	b.ReportAllocs()

	repo, nsr := newBenchRepo(b)
	ctx := b.Context()
	seedNamespace(b, nsr, "ns")

	const path = "/svc/history.json"

	if err := repo.Create(ctx, newTestConfig("ns", path, "v0")); err != nil {
		b.Fatal(err)
	}

	cfg, err := repo.Get(ctx, path, "ns")
	if err != nil {
		b.Fatal(err)
	}

	for i := range 50 {
		cfg.Content = "v" + strconv.Itoa(i+1)
		if err := repo.Update(ctx, cfg); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()

	for b.Loop() {
		if _, err := repo.GetConfigHistory(ctx, path, "ns", 20); err != nil {
			b.Fatal(err)
		}
	}
}

// seedConfigs creates n configs under the /svc/ prefix and returns their
// paths, so a benchmark that consumes one per iteration can index into them
// without formatting keys inside the timed loop.
func seedConfigs(b *testing.B, repo *configrepo.Repository, namespace string, n int) []string {
	b.Helper()

	paths := benchKeys(n)
	content := string(benchPayload(256))

	for _, p := range paths {
		if err := repo.Create(b.Context(), newTestConfig(namespace, p, content)); err != nil {
			b.Fatal(err)
		}
	}

	return paths
}
