//go:build integration

// The acceptance suite for leases: etcd's own concurrency recipes, unmodified,
// against Elara. Locks and leader election are the reason clients reach for an
// etcd-compatible store, and they exercise leases, atomic Txn, prefix Range with
// a create-revision sort, and Watch together — which is why nothing smaller
// proves the feature works.
package etcdv3_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"

	itest "github.com/sergeyslonimsky/elara/test/integration"
)

func TestIntegration_Recipes_SessionGrantsAndKeepsALease(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	s.StartLeaseExpirer(t)
	ctx := t.Context()

	sess, err := concurrency.NewSession(cli, concurrency.WithTTL(1), concurrency.WithContext(ctx))
	require.NoError(t, err)

	defer func() { _ = sess.Close() }()

	_, err = cli.Put(ctx, "/prod/session-held.json", "v", clientv3.WithLease(sess.Lease()))
	require.NoError(t, err)

	// Well beyond the TTL: the session's own keepalive is what keeps this alive.
	time.Sleep(3 * time.Second)

	resp, err := cli.Get(ctx, "/prod/session-held.json")
	require.NoError(t, err)
	assert.Len(t, resp.Kvs, 1, "a live session holds its keys open")
}

func TestIntegration_Recipes_MutexSerializesTwoHolders(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	s.StartLeaseExpirer(t)
	ctx := t.Context()

	first, err := concurrency.NewSession(cli, concurrency.WithTTL(5), concurrency.WithContext(ctx))
	require.NoError(t, err)

	defer func() { _ = first.Close() }()

	second, err := concurrency.NewSession(cli, concurrency.WithTTL(5), concurrency.WithContext(ctx))
	require.NoError(t, err)

	defer func() { _ = second.Close() }()

	held := concurrency.NewMutex(first, "/prod/mu")
	require.NoError(t, held.Lock(ctx))

	// The second holder must not get in while the first has it.
	acquired := make(chan struct{})
	waiting := concurrency.NewMutex(second, "/prod/mu")

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		if err := waiting.Lock(ctx); err == nil {
			close(acquired)
		}
	}()

	select {
	case <-acquired:
		t.Fatal("two holders took the same mutex")
	case <-time.After(500 * time.Millisecond):
	}

	require.NoError(t, held.Unlock(ctx))

	select {
	case <-acquired:
	case <-time.After(10 * time.Second):
		t.Fatal("the waiting holder never acquired the mutex after it was released")
	}

	wg.Wait()
	require.NoError(t, waiting.Unlock(ctx))
}

func TestIntegration_Recipes_ElectionElectsALeader(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	s.StartLeaseExpirer(t)
	ctx := t.Context()

	sess, err := concurrency.NewSession(cli, concurrency.WithTTL(5), concurrency.WithContext(ctx))
	require.NoError(t, err)

	defer func() { _ = sess.Close() }()

	election := concurrency.NewElection(sess, "/prod/leader")
	require.NoError(t, election.Campaign(ctx, "node-a"))

	leader, err := election.Leader(ctx)
	require.NoError(t, err)
	require.Len(t, leader.Kvs, 1)
	assert.Equal(t, "node-a", string(leader.Kvs[0].Value))

	require.NoError(t, election.Resign(ctx))

	_, err = election.Leader(ctx)
	require.ErrorIs(t, err, concurrency.ErrElectionNoLeader,
		"resigning leaves the election without a leader")
}

func TestIntegration_Recipes_LeadershipMovesWhenALeaderDies(t *testing.T) {
	t.Parallel()

	// The scenario the whole epic is for: a leader stops renewing — its process
	// died — and leadership moves once its lease runs out. Nothing revokes
	// anything explicitly here; the expiry sweep is what breaks the tie.
	s := itest.New(t)
	cli := s.StartEtcd(t)
	s.StartLeaseExpirer(t)
	ctx := t.Context()

	dying, err := concurrency.NewSession(cli, concurrency.WithTTL(1), concurrency.WithContext(ctx))
	require.NoError(t, err)

	survivor, err := concurrency.NewSession(cli, concurrency.WithTTL(10), concurrency.WithContext(ctx))
	require.NoError(t, err)

	defer func() { _ = survivor.Close() }()

	first := concurrency.NewElection(dying, "/prod/failover")
	require.NoError(t, first.Campaign(ctx, "node-a"))

	second := concurrency.NewElection(survivor, "/prod/failover")
	elected := make(chan error, 1)

	go func() {
		elected <- second.Campaign(ctx, "node-b")
	}()

	// Still node-a's: the challenger waits rather than taking over.
	select {
	case err := <-elected:
		t.Fatalf("a second candidate finished campaigning while the leader still held the lease: %v", err)
	case <-time.After(300 * time.Millisecond):
	}

	// Orphan stops the keepalive without revoking — as close to a process dying
	// as a test can get. From here only the TTL running out can move leadership.
	dying.Orphan()

	// Localise a failure: if the leader's key is still here, the sweep is at
	// fault; if it is gone and leadership still has not moved, the waiting side
	// is.
	require.Eventually(t, func() bool {
		resp, err := cli.Get(ctx, "/prod/failover", clientv3.WithPrefix())

		return err == nil && len(resp.Kvs) == 1 && string(resp.Kvs[0].Value) == "node-b"
	}, 15*time.Second, 50*time.Millisecond, "the dead leader's key never expired")

	select {
	case err := <-elected:
		require.NoError(t, err, "the challenger's campaign failed instead of winning")
	case <-time.After(15 * time.Second):
		t.Fatal("leadership never moved after the leader's lease expired")
	}

	leader, err := second.Leader(ctx)
	require.NoError(t, err)
	require.Len(t, leader.Kvs, 1)
	assert.Equal(t, "node-b", string(leader.Kvs[0].Value))
}
