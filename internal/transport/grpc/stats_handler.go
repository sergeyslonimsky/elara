package grpc

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// ClientRegistry is the surface the stats handler needs from the monitor.
// Defined here (not imported from monitor) to keep transport free of circular
// deps and to make the handler easy to test with a fake.
type ClientRegistry interface {
	RegisterConnection(info domain.ConnectionInfo) string
	UpdateIdentity(connID string, info domain.ConnectionInfo)
	UnregisterConnection(connID string)
	RecordRequest(connID, method, key string, revision int64, duration time.Duration, err error)
}

// Header keys for client-supplied identity. All lowercase per gRPC metadata
// convention.
const (
	headerClientName    = "x-client-name"
	headerClientVersion = "x-client-version"
	headerK8sNamespace  = "x-client-k8s-namespace"
	headerK8sPod        = "x-client-k8s-pod"
	headerK8sNode       = "x-client-k8s-node"
	headerInstanceID    = "x-client-instance-id"
)

// connIDKey and rpcStartKey are private context keys.
type ctxKey int

const (
	connIDKey ctxKey = iota
	rpcStartKey
)

// ConnIDFromContext returns the connection ID assigned to the current RPC's
// underlying connection. Returns "" if the context did not flow through the
// stats handler (e.g., in tests).
func ConnIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(connIDKey).(string)

	return v
}

// StatsHandler implements google.golang.org/grpc/stats.Handler and bridges
// gRPC connection/RPC lifecycle to a ClientRegistry.
type StatsHandler struct {
	reg ClientRegistry
}

func NewStatsHandler(reg ClientRegistry) *StatsHandler {
	return &StatsHandler{reg: reg}
}

// Compile-time check.
var _ stats.Handler = (*StatsHandler)(nil)

// TagConn assigns a fresh connection ID and stashes it in the conn-level context.
func (h *StatsHandler) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	peer := ""
	if info != nil && info.RemoteAddr != nil {
		peer = info.RemoteAddr.String()
	}

	id := h.reg.RegisterConnection(domain.ConnectionInfo{PeerAddress: peer})

	return context.WithValue(ctx, connIDKey, id)
}

// HandleConn fires on connection-level events. We use ConnEnd to unregister.
// (Registration happened in TagConn so the ID is available immediately for
// the first RPC.)
func (h *StatsHandler) HandleConn(ctx context.Context, s stats.ConnStats) {
	if _, isEnd := s.(*stats.ConnEnd); !isEnd {
		return
	}

	connID := ConnIDFromContext(ctx)
	if connID == "" {
		return
	}

	h.reg.UnregisterConnection(connID)
}

// TagRPC reads incoming metadata and lazily updates the client identity.
// Records RPC start time so HandleRPC(End) can compute duration without a map.
func (h *StatsHandler) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	connID := ConnIDFromContext(ctx)
	if connID != "" {
		if ident := identityFromMetadata(ctx); hasAnyIdentityField(ident) {
			h.reg.UpdateIdentity(connID, ident)
		}
	}

	method := ""
	if info != nil {
		method = info.FullMethodName
	}

	ctx = context.WithValue(ctx, grpcMethodKey, method)

	return context.WithValue(ctx, rpcStartKey, time.Now())
}

// HandleRPC fires on per-RPC events. We record completion stats on End.
func (h *StatsHandler) HandleRPC(ctx context.Context, s stats.RPCStats) {
	end, ok := s.(*stats.End)
	if !ok {
		return
	}

	connID := ConnIDFromContext(ctx)
	if connID == "" {
		return
	}

	start, _ := ctx.Value(rpcStartKey).(time.Time)

	dur := time.Duration(0)
	if !start.IsZero() {
		dur = end.EndTime.Sub(start)
	}

	method := methodFromContext(ctx)
	h.reg.RecordRequest(connID, method, "" /*key — populated by per-RPC code if desired*/, 0, dur, end.Error)
}

// identityFromMetadata extracts client identity headers from incoming context.
func identityFromMetadata(ctx context.Context) domain.ConnectionInfo {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return domain.ConnectionInfo{}
	}

	first := func(key string) string {
		vs := md.Get(key)
		if len(vs) == 0 {
			return ""
		}

		return strings.TrimSpace(vs[0])
	}

	return domain.ConnectionInfo{
		UserAgent:     first("user-agent"),
		ClientName:    first(headerClientName),
		ClientVersion: first(headerClientVersion),
		K8sNamespace:  first(headerK8sNamespace),
		K8sPod:        first(headerK8sPod),
		K8sNode:       first(headerK8sNode),
		InstanceID:    first(headerInstanceID),
	}
}

func hasAnyIdentityField(info domain.ConnectionInfo) bool {
	return info.UserAgent != "" ||
		info.ClientName != "" ||
		info.ClientVersion != "" ||
		info.K8sNamespace != "" ||
		info.K8sPod != "" ||
		info.K8sNode != "" ||
		info.InstanceID != ""
}

// methodFromContext returns the gRPC FullMethod for the current RPC.
// gRPC stashes it in the RPC context as "grpc-method" via the standard
// stats.RPCTagInfo path. We keep this best-effort — empty string if unknown.
func methodFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(grpcMethodKey).(string); ok {
		return v
	}

	return ""
}

// grpcMethodKey is set by TagRPC via the FullMethodName from RPCTagInfo.
// Defined separately to keep TagRPC's code path clean above.
type grpcMethodKeyT struct{}

//nolint:gochecknoglobals // context key — standard Go pattern
var grpcMethodKey grpcMethodKeyT
