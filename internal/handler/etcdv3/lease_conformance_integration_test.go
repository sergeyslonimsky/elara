//go:build integration

// Lease conformance against a real clientv3 over the etcd-compatible server.
// These are the behaviours lock and leader-election recipes depend on: a key
// dies with its lease, keepalive keeps it alive, and revoke kills it at once.
package etcdv3_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"

	itest "github.com/sergeyslonimsky/elara/test/integration"
)

func TestIntegration_Lease_KeyCarriesItsLease(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	grant, err := cli.Grant(ctx, 60)
	require.NoError(t, err)
	require.NotZero(t, grant.ID)

	_, err = cli.Put(ctx, "/prod/lock.json", "mine", clientv3.WithLease(grant.ID))
	require.NoError(t, err)

	resp, err := cli.Get(ctx, "/prod/lock.json")
	require.NoError(t, err)
	require.Len(t, resp.Kvs, 1)
	assert.Equal(t, int64(grant.ID), resp.Kvs[0].Lease,
		"a key read back reports the lease it was written under")
}

func TestIntegration_Lease_ExpiryDeletesAttachedKeys(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	s.StartLeaseExpirer(t)
	ctx := t.Context()

	grant, err := cli.Grant(ctx, 1)
	require.NoError(t, err)

	_, err = cli.Put(ctx, "/prod/ephemeral.json", "mine", clientv3.WithLease(grant.ID))
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		resp, err := cli.Get(ctx, "/prod/ephemeral.json")

		return err == nil && len(resp.Kvs) == 0
	}, 10*time.Second, 50*time.Millisecond, "the key must go when its lease expires")

	// The lease itself is gone too, not merely its keys.
	ttl, err := cli.TimeToLive(ctx, grant.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(-1), ttl.TTL, "-1 is the wire's way of saying there is no such lease")
}

func TestIntegration_Lease_ExpiryPublishesADeleteToWatchers(t *testing.T) {
	t.Parallel()

	// Waiting for a lock or an election is built on watching for the previous
	// holder's key to disappear, so an expiry that deletes silently would leave
	// every waiter blocked forever.
	s := itest.New(t)
	cli := s.StartEtcd(t)
	s.StartLeaseExpirer(t)
	ctx := t.Context()

	grant, err := cli.Grant(ctx, 1)
	require.NoError(t, err)

	_, err = cli.Put(ctx, "/prod/watched/key.json", "v", clientv3.WithLease(grant.ID))
	require.NoError(t, err)

	watchCh := cli.Watch(ctx, "/prod/watched/", clientv3.WithPrefix())

	select {
	case resp := <-watchCh:
		require.NotEmpty(t, resp.Events)
		assert.Equal(t, clientv3.EventTypeDelete, resp.Events[0].Type)
		assert.Equal(t, "/prod/watched/key.json", string(resp.Events[0].Kv.Key))
	case <-time.After(10 * time.Second):
		t.Fatal("no delete event reached the watcher when the lease expired")
	}
}

func TestIntegration_Lease_KeepAliveHoldsTheKey(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	s.StartLeaseExpirer(t)
	ctx := t.Context()

	grant, err := cli.Grant(ctx, 1)
	require.NoError(t, err)

	_, err = cli.Put(ctx, "/prod/held.json", "mine", clientv3.WithLease(grant.ID))
	require.NoError(t, err)

	// KeepAlive runs until ctx ends; the client pings at roughly TTL/3.
	ka, err := cli.KeepAlive(ctx, grant.ID)
	require.NoError(t, err)

	// Drain the channel so the client keeps renewing rather than blocking.
	go func() {
		for range ka { //nolint:revive // draining is the point
		}
	}()

	// Well past the TTL: without renewals the sweep would have taken the key.
	time.Sleep(3 * time.Second)

	resp, err := cli.Get(ctx, "/prod/held.json")
	require.NoError(t, err)
	assert.Len(t, resp.Kvs, 1, "a renewed lease keeps its key")
}

// TestIntegration_Range_MaxCreateRevisionExcludesTheCallersOwnKey is the exact
// query clientv3's waitDeletes issues while waiting its turn: "the newest key in
// this prefix created strictly before mine". Getting it wrong makes a waiter
// wait on its own key, which never goes away.
func TestIntegration_Range_MaxCreateRevisionExcludesTheCallersOwnKey(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	const prefix = "/prod/waiters/"

	first, err := cli.Put(ctx, prefix+"a", "1")
	require.NoError(t, err)

	mine, err := cli.Put(ctx, prefix+"b", "2")
	require.NoError(t, err)

	myRev := mine.Header.Revision

	opts := append(clientv3.WithLastCreate(), clientv3.WithMaxCreateRev(myRev-1))

	resp, err := cli.Get(ctx, prefix, opts...)
	require.NoError(t, err)
	require.Len(t, resp.Kvs, 1, "exactly the one predecessor")
	assert.Equal(t, prefix+"a", string(resp.Kvs[0].Key))
	assert.Equal(t, first.Header.Revision, resp.Kvs[0].CreateRevision)

	// And once the predecessor is gone, the same query must come back empty
	// rather than falling back to the caller's own key.
	_, err = cli.Delete(ctx, prefix+"a")
	require.NoError(t, err)

	resp, err = cli.Get(ctx, prefix, opts...)
	require.NoError(t, err)
	assert.Empty(t, resp.Kvs, "a waiter must not be left waiting on its own key")
}

// TestIntegration_Watch_SingleKeyFromRevisionSeesDelete is the other half of
// waitDeletes: having found its predecessor, a waiter watches that one key from
// the revision it read, and waits for the delete.
func TestIntegration_Watch_SingleKeyFromRevisionSeesDelete(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	const key = "/prod/watched-single.json"

	_, err := cli.Put(ctx, key, "v")
	require.NoError(t, err)

	got, err := cli.Get(ctx, key)
	require.NoError(t, err)

	watchCh := cli.Watch(ctx, key, clientv3.WithRev(got.Header.Revision))

	_, err = cli.Delete(ctx, key)
	require.NoError(t, err)

	deadline := time.After(10 * time.Second)

	for {
		select {
		case resp := <-watchCh:
			for _, ev := range resp.Events {
				if ev.Type == clientv3.EventTypeDelete {
					return
				}
			}
		case <-deadline:
			t.Fatal("watching a single key from a revision never delivered its delete")
		}
	}
}

// TestIntegration_Txn_HeaderRevisionMatchesThePutItMade pins the assumption every
// lock and election recipe makes: after a Txn whose Then branch put a key, the
// response header's revision *is* that key's create revision. Recipes derive
// "keys older than mine" from header revision minus one, so a header that runs
// ahead makes a waiter wait on its own key forever.
func TestIntegration_Txn_HeaderRevisionMatchesThePutItMade(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	grant, err := cli.Grant(ctx, 60)
	require.NoError(t, err)

	tests := []struct {
		name string
		key  string
		put  func(key string) clientv3.Op
	}{
		{
			name: "plain put",
			key:  "/prod/txn-rev.json",
			put:  func(key string) clientv3.Op { return clientv3.OpPut(key, "v") },
		},
		{
			// What every recipe actually issues: the campaign key is written
			// under the session's lease.
			name: "put under a lease",
			key:  "/prod/txn-rev-leased.json",
			put: func(key string) clientv3.Op {
				return clientv3.OpPut(key, "v", clientv3.WithLease(grant.ID))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp, err := cli.Txn(ctx).
				If(clientv3.Compare(clientv3.CreateRevision(tt.key), "=", 0)).
				Then(tt.put(tt.key)).
				Else(clientv3.OpGet(tt.key)).
				Commit()
			require.NoError(t, err)
			require.True(t, resp.Succeeded)

			got, err := cli.Get(ctx, tt.key)
			require.NoError(t, err)
			require.Len(t, got.Kvs, 1)

			assert.Equal(t, got.Kvs[0].CreateRevision, resp.Header.Revision,
				"the Txn header must report the revision its own Put created")
		})
	}
}

// TestIntegration_Watch_SingleKeySeesAnExpiryDelete is the combination the
// failover path actually needs: one waiter watching one key, and the delete
// coming from the expiry sweep rather than from a client call.
func TestIntegration_Watch_SingleKeySeesAnExpiryDelete(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	s.StartLeaseExpirer(t)
	ctx := t.Context()

	const key = "/prod/expiring-single.json"

	grant, err := cli.Grant(ctx, 1)
	require.NoError(t, err)

	_, err = cli.Put(ctx, key, "v", clientv3.WithLease(grant.ID))
	require.NoError(t, err)

	got, err := cli.Get(ctx, key)
	require.NoError(t, err)

	watchCh := cli.Watch(ctx, key, clientv3.WithRev(got.Header.Revision))

	deadline := time.After(10 * time.Second)

	for {
		select {
		case resp := <-watchCh:
			for _, ev := range resp.Events {
				if ev.Type == clientv3.EventTypeDelete {
					// A watcher that learns of a delete before the transaction
					// carrying it commits would read the key back and wait for a
					// second event that never comes — which is exactly how a
					// waiting lock or election hangs.
					got, err := cli.Get(ctx, key)
					require.NoError(t, err)
					assert.Empty(t, got.Kvs,
						"the key must already be gone when its delete reaches a watcher")

					return
				}
			}
		case <-deadline:
			t.Fatal("a single-key watcher never saw the expiry delete")
		}
	}
}
// TestIntegration_Election_WaitQueryAfterTheLeaderExpires replays what a losing
// candidate does, one call at a time, so a failure points at the step rather
// than at "the recipe hung".
func TestIntegration_Election_WaitQueryAfterTheLeaderExpires(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	s.StartLeaseExpirer(t)
	ctx := t.Context()

	const prefix = "/prod/manual-election"

	leaderLease, err := cli.Grant(ctx, 1)
	require.NoError(t, err)

	challengerLease, err := cli.Grant(ctx, 60)
	require.NoError(t, err)

	leaderKey := fmt.Sprintf("%s%x", prefix, leaderLease.ID)
	challengerKey := fmt.Sprintf("%s%x", prefix, challengerLease.ID)

	leaderTxn, err := cli.Txn(ctx).
		If(clientv3.Compare(clientv3.CreateRevision(leaderKey), "=", 0)).
		Then(clientv3.OpPut(leaderKey, "node-a", clientv3.WithLease(leaderLease.ID))).
		Commit()
	require.NoError(t, err)
	require.True(t, leaderTxn.Succeeded)

	challengerTxn, err := cli.Txn(ctx).
		If(clientv3.Compare(clientv3.CreateRevision(challengerKey), "=", 0)).
		Then(clientv3.OpPut(challengerKey, "node-b", clientv3.WithLease(challengerLease.ID))).
		Commit()
	require.NoError(t, err)
	require.True(t, challengerTxn.Succeeded)

	waitOpts := append(
		clientv3.WithLastCreate(),
		clientv3.WithMaxCreateRev(challengerTxn.Header.Revision-1),
	)

	// While the leader lives, the challenger sees exactly one predecessor.
	resp, err := cli.Get(ctx, prefix, waitOpts...)
	require.NoError(t, err)
	require.Len(t, resp.Kvs, 1)
	require.Equal(t, leaderKey, string(resp.Kvs[0].Key))

	// The leader is never renewed, so its lease runs out and the sweep takes its
	// key. After that the same query must come back empty — that emptiness is
	// precisely what tells the challenger it has won.
	require.Eventually(t, func() bool {
		resp, err := cli.Get(ctx, prefix, clientv3.WithPrefix())

		return err == nil && len(resp.Kvs) == 1
	}, 15*time.Second, 50*time.Millisecond, "the leader's key never expired")

	resp, err = cli.Get(ctx, prefix, waitOpts...)
	require.NoError(t, err)
	assert.Empty(t, resp.Kvs, "with the leader gone, the challenger must see no predecessor")
}

func TestIntegration_Lease_RevokeDeletesKeysAtOnce(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	grant, err := cli.Grant(ctx, 60)
	require.NoError(t, err)

	// A prefix of its own: the suite seeds demo data into /prod, so a broader
	// scan would assert on keys this test never wrote.
	const prefix = "/prod/revoked/"

	for _, key := range []string{prefix + "a.json", prefix + "b.json"} {
		_, err = cli.Put(ctx, key, "v", clientv3.WithLease(grant.ID))
		require.NoError(t, err)
	}

	before, err := cli.Get(ctx, prefix, clientv3.WithPrefix())
	require.NoError(t, err)
	require.Len(t, before.Kvs, 2)

	_, err = cli.Revoke(ctx, grant.ID)
	require.NoError(t, err)

	after, err := cli.Get(ctx, prefix, clientv3.WithPrefix())
	require.NoError(t, err)
	assert.Empty(t, after.Kvs, "revoking takes every key the lease held")

	// One revision for the whole set, not one per key.
	assert.Equal(t, before.Header.Revision+1, after.Header.Revision)
}

func TestIntegration_Lease_TimeToLiveListsItsKeys(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	grant, err := cli.Grant(ctx, 60)
	require.NoError(t, err)

	_, err = cli.Put(ctx, "/prod/listed.json", "v", clientv3.WithLease(grant.ID))
	require.NoError(t, err)

	ttl, err := cli.TimeToLive(ctx, grant.ID, clientv3.WithAttachedKeys())
	require.NoError(t, err)
	assert.Equal(t, int64(60), ttl.GrantedTTL)
	assert.Positive(t, ttl.TTL)
	require.Len(t, ttl.Keys, 1)
	assert.Equal(t, "/prod/listed.json", string(ttl.Keys[0]))
}

func TestIntegration_Lease_PutUnderAnUnknownLeaseIsRejected(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	_, err := cli.Put(ctx, "/prod/orphan.json", "v", clientv3.WithLease(clientv3.LeaseID(123456)))
	require.Error(t, err, "attaching to a lease that does not exist must not silently succeed")
}
