//go:build integration

// Lease conformance against a real clientv3 over the etcd-compatible server.
// These are the behaviours lock and leader-election recipes depend on: a key
// dies with its lease, keepalive keeps it alive, and revoke kills it at once.
package etcdv3_test

import (
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
