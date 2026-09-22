//go:build integration

// Ш0 baseline (plans/etcd-usecase-decoupling/plan.md): a real round-trip
// suite through clientv3 against the actual etcd-compatible gRPC server,
// not the stub-repo-based unit tests in kv_server_test.go. This captures
// current KV wire-protocol behavior so Ш1 (moving Put/Range/DeleteRange
// onto the usecase layer) has a byte-for-byte baseline to diff against.
//
// Lock recipes are exercised via raw Txn compare-and-swap, not clientv3's
// concurrency package: that package requires lease sessions, and
// LeaseServer isn't registered by service.EtcdRoutes (see plan's
// Верификация section — leases are explicitly out of scope here).
package etcdv3_test

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"

	itest "github.com/sergeyslonimsky/elara/test/integration"
)

func TestIntegration_KVConformance_PutGetByteIdentity(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	// Binary, non-UTF8 payload — etcd values are opaque bytes, not
	// necessarily JSON/text (locks, leader-election payloads, etc).
	value := []byte{0x00, 0xFF, 0xFE, 'h', 'i', 0x00}

	_, err := cli.Put(ctx, "/prod/bin.dat", string(value))
	require.NoError(t, err)

	resp, err := cli.Get(ctx, "/prod/bin.dat")
	require.NoError(t, err)
	require.Len(t, resp.Kvs, 1)
	assert.Equal(t, value, resp.Kvs[0].Value, "value must round-trip byte-for-byte")
	assert.Equal(t, "/prod/bin.dat", string(resp.Kvs[0].Key))
}

func TestIntegration_KVConformance_NamespaceAutoVivify(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	// "never-seeded" was never created via NamespaceRepo — current etcd Put
	// behavior auto-vivifies rather than rejecting like the ConnectRPC path.
	// Ш1 explicitly treats changing this as a deliberate, separately
	// documented decision (see plan's Ш1 "Namespace-existence check" note),
	// not something to fold in silently.
	_, err := cli.Put(ctx, "/never-seeded/x.json", `{"a":1}`)
	require.NoError(t, err, "current behavior: unseeded namespaces auto-vivify on etcd Put")

	resp, err := cli.Get(ctx, "/never-seeded/x.json")
	require.NoError(t, err)
	require.Len(t, resp.Kvs, 1)
	assert.Equal(t, `{"a":1}`, string(resp.Kvs[0].Value))
}

func TestIntegration_KVConformance_PrefixRange(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	keys := []string{"/prod/svc/a.json", "/prod/svc/b.json", "/prod/svc/c/d.json"}
	for _, k := range keys {
		_, err := cli.Put(ctx, k, k)
		require.NoError(t, err)
	}
	// Outside the prefix — must not be returned.
	_, err := cli.Put(ctx, "/prod/other.json", "excluded")
	require.NoError(t, err)

	resp, err := cli.Get(ctx, "/prod/svc/", clientv3.WithPrefix())
	require.NoError(t, err)

	got := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		got = append(got, string(kv.Key))
	}
	assert.ElementsMatch(t, keys, got)
}

func TestIntegration_KVConformance_FullKeyspaceScan(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	// range_end == \x00 (clientv3.WithFromKey) means "every key >= start",
	// crossing namespace boundaries — distinct from a single-namespace
	// prefix range.
	_, err := cli.Put(ctx, "/prod/a.json", "prod-a")
	require.NoError(t, err)
	_, err = cli.Put(ctx, "/staging/a.json", "staging-a")
	require.NoError(t, err)

	resp, err := cli.Get(ctx, "/prod/a.json", clientv3.WithFromKey())
	require.NoError(t, err)

	got := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		got = append(got, string(kv.Key))
	}
	assert.Contains(t, got, "/prod/a.json")
	assert.Contains(t, got, "/staging/a.json", "scanAll must cross namespace boundaries")
}

func TestIntegration_KVConformance_HistoricalReadByRevision(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	putV1, err := cli.Put(ctx, "/prod/versioned.json", "v1")
	require.NoError(t, err)
	rev1 := putV1.Header.Revision

	_, err = cli.Put(ctx, "/prod/versioned.json", "v2")
	require.NoError(t, err)

	// Read at the historical revision must return the old value, not the
	// current one.
	resp, err := cli.Get(ctx, "/prod/versioned.json", clientv3.WithRev(rev1))
	require.NoError(t, err)
	require.Len(t, resp.Kvs, 1)
	assert.Equal(t, "v1", string(resp.Kvs[0].Value))

	current, err := cli.Get(ctx, "/prod/versioned.json")
	require.NoError(t, err)
	require.Len(t, current.Kvs, 1)
	assert.Equal(t, "v2", string(current.Kvs[0].Value))
}

func TestIntegration_KVConformance_PrevKv(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	_, err := cli.Put(ctx, "/prod/prevkv.json", "before")
	require.NoError(t, err)

	putResp, err := cli.Put(ctx, "/prod/prevkv.json", "after", clientv3.WithPrevKV())
	require.NoError(t, err)
	require.NotNil(t, putResp.PrevKv, "PrevKv must be populated on overwrite")
	assert.Equal(t, "before", string(putResp.PrevKv.Value))

	delResp, err := cli.Delete(ctx, "/prod/prevkv.json", clientv3.WithPrevKV())
	require.NoError(t, err)
	require.Len(t, delResp.PrevKvs, 1)
	assert.Equal(t, "after", string(delResp.PrevKvs[0].Value))
}

// TestIntegration_KVConformance_TxnCompareAndSwap exercises the CAS pattern
// distributed locks are built on (compare mod_revision, then put/get) — the
// primitive underneath lock recipes, given lease-based sessions aren't
// available on this server (see file doc comment).
func TestIntegration_KVConformance_TxnCompareAndSwap(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	putResp, err := cli.Put(ctx, "/prod/lock.key", "v1")
	require.NoError(t, err)
	rev := putResp.Header.Revision

	// Compare against the correct mod_revision — must succeed.
	txnResp, err := cli.Txn(ctx).
		If(clientv3.Compare(clientv3.ModRevision("/prod/lock.key"), "=", rev)).
		Then(clientv3.OpPut("/prod/lock.key", "v2")).
		Commit()
	require.NoError(t, err)
	assert.True(t, txnResp.Succeeded)

	// Same (now stale) revision must fail — this is the CAS guarantee a
	// lock recipe depends on to avoid two holders succeeding at once.
	staleResp, err := cli.Txn(ctx).
		If(clientv3.Compare(clientv3.ModRevision("/prod/lock.key"), "=", rev)).
		Then(clientv3.OpPut("/prod/lock.key", "v3")).
		Commit()
	require.NoError(t, err)
	assert.False(t, staleResp.Succeeded, "stale compare must not succeed")

	get, err := cli.Get(ctx, "/prod/lock.key")
	require.NoError(t, err)
	require.Len(t, get.Kvs, 1)
	assert.Equal(t, "v2", string(get.Kvs[0].Value), "the failed Txn must not have applied its Then ops")
}

// TestIntegration_KVConformance_TxnCompareAndSwap_Concurrent races N clients
// doing the create-if-absent CAS pattern against the same key. This
// documents CURRENT (broken) behavior, not desired behavior: Txn's compare
// and write run in separate bbolt transactions (kv_server.go admits this in
// its own comment), so under real concurrency more than one racer can see
// CreateRevision==0 and "win" — confirmed empirically here, not just from
// the comment. Only ">= 1 winner" is a safe baseline assertion; the exact
// count is nondeterministic race-timing, not a stable invariant.
//
// Once Ш1 wraps this path in storage.Manager.WithTx, tighten the assertion
// below to require exactly one winner — that flip is the atomicity
// regression test for Ш1.
func TestIntegration_KVConformance_TxnCompareAndSwap_Concurrent(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	const racers = 8

	var wins atomic.Int32

	results := make(chan error, racers)
	for range racers {
		go func() {
			resp, err := cli.Txn(ctx).
				If(clientv3.Compare(clientv3.CreateRevision("/prod/racer.key"), "=", 0)).
				Then(clientv3.OpPut("/prod/racer.key", "winner")).
				Commit()
			if err != nil {
				results <- err

				return
			}

			if resp.Succeeded {
				wins.Add(1)
			}

			results <- nil
		}()
	}

	for range racers {
		require.NoError(t, <-results)
	}

	t.Logf("racers that won the CAS: %d/%d (non-atomic Txn baseline, see kv_server.go:202-206)", wins.Load(), racers)
	assert.GreaterOrEqual(t, wins.Load(), int32(1), "at least one create-if-absent racer must win")

	get, err := cli.Get(ctx, "/prod/racer.key")
	require.NoError(t, err)
	require.Len(t, get.Kvs, 1)
	assert.Equal(t, "winner", string(get.Kvs[0].Value))
}
