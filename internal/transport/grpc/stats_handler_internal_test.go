package grpc

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// fakeRegistry records every call for assertions.
type fakeRegistry struct {
	mu sync.Mutex

	registered   []domain.ConnectionInfo
	identityMu   sync.Mutex
	identity     map[string]domain.ConnectionInfo
	unregistered []string
	requests     []reqRecord
	nextID       int
}

type reqRecord struct {
	connID, method, key string
	revision            int64
	duration            time.Duration
	err                 error
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{identity: make(map[string]domain.ConnectionInfo)}
}

func (r *fakeRegistry) RegisterConnection(info domain.ConnectionInfo) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.registered = append(r.registered, info)
	r.nextID++

	return "conn-" + itoa(r.nextID)
}

func (r *fakeRegistry) UpdateIdentity(connID string, info domain.ConnectionInfo) {
	r.identityMu.Lock()
	defer r.identityMu.Unlock()
	r.identity[connID] = info
}

func (r *fakeRegistry) UnregisterConnection(connID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unregistered = append(r.unregistered, connID)
}

func (r *fakeRegistry) RecordRequest(connID, method, key string, rev int64, dur time.Duration, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, reqRecord{connID, method, key, rev, dur, err})
}

func (r *fakeRegistry) snapshot() ([]domain.ConnectionInfo, []string, []reqRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()

	regs := make([]domain.ConnectionInfo, len(r.registered))
	copy(regs, r.registered)

	uns := make([]string, len(r.unregistered))
	copy(uns, r.unregistered)

	rs := make([]reqRecord, len(r.requests))
	copy(rs, r.requests)

	return regs, uns, rs
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	var buf []byte
	for i > 0 {
		buf = append([]byte{byte('0' + i%10)}, buf...)
		i /= 10
	}

	return string(buf)
}

// fakeAddr satisfies net.Addr.
type fakeAddr struct{ s string }

func (a fakeAddr) Network() string { return "tcp" }
func (a fakeAddr) String() string  { return a.s }

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

func TestStatsHandler_TagConn_RegistersAndPropagatesID(t *testing.T) {
	t.Parallel()

	reg := newFakeRegistry()
	h := NewStatsHandler(reg)

	ctx := h.TagConn(context.Background(), &stats.ConnTagInfo{
		RemoteAddr: fakeAddr{"10.0.0.5:54321"},
	})

	regs, _, _ := reg.snapshot()
	require.Len(t, regs, 1)
	assert.Equal(t, "10.0.0.5:54321", regs[0].PeerAddress)

	id := ConnIDFromContext(ctx)
	assert.Equal(t, "conn-1", id, "ID must be retrievable from returned ctx")
}

func TestStatsHandler_TagConn_NilRemoteAddr(t *testing.T) {
	t.Parallel()

	reg := newFakeRegistry()
	h := NewStatsHandler(reg)

	// Must not panic on nil ConnTagInfo.RemoteAddr.
	ctx := h.TagConn(context.Background(), &stats.ConnTagInfo{})

	regs, _, _ := reg.snapshot()
	require.Len(t, regs, 1)
	assert.Empty(t, regs[0].PeerAddress)
	assert.Equal(t, "conn-1", ConnIDFromContext(ctx))
}

func TestStatsHandler_HandleConn_ConnEnd_Unregisters(t *testing.T) {
	t.Parallel()

	reg := newFakeRegistry()
	h := NewStatsHandler(reg)

	ctx := h.TagConn(context.Background(), &stats.ConnTagInfo{RemoteAddr: fakeAddr{"p"}})

	h.HandleConn(ctx, &stats.ConnEnd{})

	_, uns, _ := reg.snapshot()
	assert.Equal(t, []string{"conn-1"}, uns)
}

func TestStatsHandler_HandleConn_ConnBegin_NoOp(t *testing.T) {
	t.Parallel()

	// We register on TagConn, not on ConnBegin. Make sure ConnBegin doesn't
	// double-register or otherwise interfere.
	reg := newFakeRegistry()
	h := NewStatsHandler(reg)

	ctx := h.TagConn(context.Background(), &stats.ConnTagInfo{RemoteAddr: fakeAddr{"p"}})
	h.HandleConn(ctx, &stats.ConnBegin{})

	regs, uns, _ := reg.snapshot()
	assert.Len(t, regs, 1, "ConnBegin must not double-register")
	assert.Empty(t, uns)
}

func TestStatsHandler_HandleConn_NoConnID_NoPanic(t *testing.T) {
	t.Parallel()

	reg := newFakeRegistry()
	h := NewStatsHandler(reg)

	// ctx without conn ID — must not panic, must be no-op.
	h.HandleConn(context.Background(), &stats.ConnEnd{})

	_, uns, _ := reg.snapshot()
	assert.Empty(t, uns)
}

func TestStatsHandler_TagRPC_UpdatesIdentityFromMetadata(t *testing.T) {
	t.Parallel()

	reg := newFakeRegistry()
	h := NewStatsHandler(reg)

	connCtx := h.TagConn(context.Background(), &stats.ConnTagInfo{RemoteAddr: fakeAddr{"p"}})

	md := metadata.New(map[string]string{
		"user-agent":             "etcd-client/3.5.0",
		"x-client-name":          "order-service",
		"x-client-version":       "1.2.3",
		"x-client-k8s-namespace": "production",
		"x-client-k8s-pod":       "order-7d8c-x4k2",
		"x-client-k8s-node":      "gke-node-abc",
		"x-client-instance-id":   "instance-uuid-1",
	})
	rpcCtx := metadata.NewIncomingContext(connCtx, md)

	rpcCtx = h.TagRPC(rpcCtx, &stats.RPCTagInfo{FullMethodName: "/svc/M"})

	reg.identityMu.Lock()
	defer reg.identityMu.Unlock()

	id := reg.identity["conn-1"]
	assert.Equal(t, "etcd-client/3.5.0", id.UserAgent)
	assert.Equal(t, "order-service", id.ClientName)
	assert.Equal(t, "1.2.3", id.ClientVersion)
	assert.Equal(t, "production", id.K8sNamespace)
	assert.Equal(t, "order-7d8c-x4k2", id.K8sPod)
	assert.Equal(t, "gke-node-abc", id.K8sNode)
	assert.Equal(t, "instance-uuid-1", id.InstanceID)

	// Method must propagate via context.
	assert.Equal(t, "/svc/M", methodFromContext(rpcCtx))
}

func TestStatsHandler_TagRPC_NoMetadata_NoUpdateIdentity(t *testing.T) {
	t.Parallel()

	reg := newFakeRegistry()
	h := NewStatsHandler(reg)

	connCtx := h.TagConn(context.Background(), &stats.ConnTagInfo{RemoteAddr: fakeAddr{"p"}})
	_ = h.TagRPC(connCtx, &stats.RPCTagInfo{FullMethodName: "/svc/M"})

	reg.identityMu.Lock()
	defer reg.identityMu.Unlock()
	_, ok := reg.identity["conn-1"]
	assert.False(t, ok, "no metadata → no identity update")
}

func TestStatsHandler_HandleRPC_End_RecordsRequestWithDuration(t *testing.T) {
	t.Parallel()

	reg := newFakeRegistry()
	h := NewStatsHandler(reg)

	connCtx := h.TagConn(context.Background(), &stats.ConnTagInfo{RemoteAddr: fakeAddr{"p"}})
	rpcCtx := h.TagRPC(connCtx, &stats.RPCTagInfo{FullMethodName: "/svc/Put"})

	// Simulate elapsed time.
	time.Sleep(2 * time.Millisecond)

	end := &stats.End{EndTime: time.Now()}
	h.HandleRPC(rpcCtx, end)

	_, _, reqs := reg.snapshot()
	require.Len(t, reqs, 1)
	assert.Equal(t, "conn-1", reqs[0].connID)
	assert.Equal(t, "/svc/Put", reqs[0].method)
	assert.Greater(t, reqs[0].duration, time.Duration(0))
}

func TestStatsHandler_HandleRPC_End_PropagatesError(t *testing.T) {
	t.Parallel()

	reg := newFakeRegistry()
	h := NewStatsHandler(reg)

	connCtx := h.TagConn(context.Background(), &stats.ConnTagInfo{RemoteAddr: fakeAddr{"p"}})
	rpcCtx := h.TagRPC(connCtx, &stats.RPCTagInfo{FullMethodName: "/svc/M"})

	rpcErr := errors.New("internal")
	h.HandleRPC(rpcCtx, &stats.End{EndTime: time.Now(), Error: rpcErr})

	_, _, reqs := reg.snapshot()
	require.Len(t, reqs, 1)
	assert.Equal(t, rpcErr, reqs[0].err)
}

func TestStatsHandler_HandleRPC_NonEnd_Ignored(t *testing.T) {
	t.Parallel()

	reg := newFakeRegistry()
	h := NewStatsHandler(reg)

	connCtx := h.TagConn(context.Background(), &stats.ConnTagInfo{RemoteAddr: fakeAddr{"p"}})
	rpcCtx := h.TagRPC(connCtx, &stats.RPCTagInfo{FullMethodName: "/svc/M"})

	h.HandleRPC(rpcCtx, &stats.Begin{})
	h.HandleRPC(rpcCtx, &stats.InHeader{})

	_, _, reqs := reg.snapshot()
	assert.Empty(t, reqs, "only End triggers RecordRequest")
}

func TestStatsHandler_HandleRPC_NoConnID_NoOp(t *testing.T) {
	t.Parallel()

	reg := newFakeRegistry()
	h := NewStatsHandler(reg)

	// ctx without TagConn → no conn ID; HandleRPC must be a no-op.
	rpcCtx := h.TagRPC(context.Background(), &stats.RPCTagInfo{FullMethodName: "/svc/M"})

	h.HandleRPC(rpcCtx, &stats.End{EndTime: time.Now()})

	_, _, reqs := reg.snapshot()
	assert.Empty(t, reqs)
}

func TestConnIDFromContext_Missing_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, ConnIDFromContext(context.Background()))
}

func TestStatsHandler_FullPath_ConnAndRPCLifecycle(t *testing.T) {
	t.Parallel()

	// End-to-end-style: simulate a full connection with two RPCs and a close.
	reg := newFakeRegistry()
	h := NewStatsHandler(reg)

	connCtx := h.TagConn(context.Background(), &stats.ConnTagInfo{
		RemoteAddr: fakeAddr{"10.0.0.5:1234"},
	})

	md := metadata.New(map[string]string{
		"user-agent":    "etcd-client/3.5.0",
		"x-client-name": "order-service",
	})
	rpcCtx := metadata.NewIncomingContext(connCtx, md)

	// RPC 1
	rpcCtx1 := h.TagRPC(rpcCtx, &stats.RPCTagInfo{FullMethodName: "/etcdserverpb.KV/Put"})
	h.HandleRPC(rpcCtx1, &stats.End{EndTime: time.Now()})

	// RPC 2 with an error
	rpcCtx2 := h.TagRPC(rpcCtx, &stats.RPCTagInfo{FullMethodName: "/etcdserverpb.KV/Range"})
	h.HandleRPC(rpcCtx2, &stats.End{EndTime: time.Now(), Error: errors.New("boom")})

	// Conn close
	h.HandleConn(connCtx, &stats.ConnEnd{})

	regs, uns, reqs := reg.snapshot()
	require.Len(t, regs, 1)
	assert.Equal(t, "10.0.0.5:1234", regs[0].PeerAddress)

	reg.identityMu.Lock()
	id := reg.identity["conn-1"]
	reg.identityMu.Unlock()
	assert.Equal(t, "order-service", id.ClientName)

	require.Len(t, reqs, 2)
	assert.Equal(t, "/etcdserverpb.KV/Put", reqs[0].method)
	assert.Equal(t, "/etcdserverpb.KV/Range", reqs[1].method)
	require.Error(t, reqs[1].err)

	require.Len(t, uns, 1)
	assert.Equal(t, "conn-1", uns[0])

	// Sanity check: ensure stats.Handler interface is satisfied at compile-time.
	var _ net.Addr = fakeAddr{}
}
