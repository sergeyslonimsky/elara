package etcdv3

import (
	"bytes"
	"context"
	"errors"
	"sort"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	configuc "github.com/sergeyslonimsky/elara/internal/usecase/config"
)

// KVUsecase is the usecase surface the KV server translates etcd wire
// requests onto. Backed by usecase/config.Service (see service_kv.go) —
// KVServer no longer talks to ConfigRepo directly.
type KVUsecase interface {
	CurrentRevisionValue(ctx context.Context) (int64, error)
	RangeQuery(
		ctx context.Context,
		startNS, startPath, endNS, endPath string,
		opts configuc.KVRangeOpts,
	) ([]*domain.KVPair, int64, bool, error)
	// RangeKVs is RangeQuery without the current-revision fetch — for
	// compare-only callers (evalCompare) that discard it anyway.
	RangeKVs(
		ctx context.Context,
		startNS, startPath, endNS, endPath string,
		opts configuc.KVRangeOpts,
	) ([]*domain.KVPair, bool, error)
	PutKey(ctx context.Context, namespace, path string, value []byte) (*domain.KVPair, int64, error)
	DeleteRangeKeys(
		ctx context.Context,
		startNS, startPath, endNS, endPath string,
		returnPrev bool,
	) ([]*domain.KVPair, int64, error)
	// Txn evaluates the guard and runs one branch atomically, publishing watch
	// events only after the commit.
	Txn(ctx context.Context, in configuc.KVTxnInput) (configuc.KVTxnResult, error)
}

// KVServer implements etcdserverpb.KVServer backed by usecase/config.
//
// It does not publish watch events. Notification is ordered against the
// transaction that produced it, so it belongs to the layer that owns the
// transaction boundary — see docs/adr/0003-responsibility-placement.md.
type KVServer struct {
	etcdserverpb.UnimplementedKVServer

	usecase KVUsecase
	metrics *kvMetrics
}

func NewKVServer(usecase KVUsecase) *KVServer {
	return &KVServer{usecase: usecase, metrics: newKVMetrics()}
}

func (s *KVServer) Range(
	ctx context.Context,
	req *etcdserverpb.RangeRequest,
) (*etcdserverpb.RangeResponse, error) {
	startNS, startPath, endNS, endPath, ok := SplitRange(req.GetKey(), req.GetRangeEnd())
	if !ok {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"invalid key encoding: %q",
			string(req.GetKey()),
		)
	}

	if err := s.checkRangeAccess(ctx, startNS, endNS, domain.ActionRead); err != nil {
		return nil, err
	}

	kvs, currentRev, more, err := s.usecase.RangeQuery(
		ctx,
		startNS, startPath,
		endNS, endPath,
		configuc.KVRangeOpts{Limit: req.GetLimit(), Revision: req.GetRevision(), KeysOnly: req.GetKeysOnly()},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "range query: %v", err)
	}

	return buildRangeResponse(req, kvs, currentRev, more), nil
}

// buildRangeResponse shapes matched pairs into a Range response. Shared with the
// Txn path so a range nested in a transaction comes back byte-identical to the
// same range issued on its own — sort order, count-only and the header all being
// properties of the request rather than of the stored data.
func buildRangeResponse(
	req *etcdserverpb.RangeRequest,
	kvs []*domain.KVPair,
	revision int64,
	more bool,
) *etcdserverpb.RangeResponse {
	protoKVs := make([]*mvccpb.KeyValue, 0, len(kvs))
	for _, kv := range kvs {
		protoKVs = append(protoKVs, kvPairToProto(kv))
	}

	sortKVs(protoKVs, req.GetSortOrder(), req.GetSortTarget())

	count := int64(len(protoKVs))
	if req.GetCountOnly() {
		protoKVs = nil
	}

	return &etcdserverpb.RangeResponse{
		Header: newHeader(revision),
		Kvs:    protoKVs,
		More:   more,
		Count:  count,
	}
}

func (s *KVServer) Put(
	ctx context.Context,
	req *etcdserverpb.PutRequest,
) (*etcdserverpb.PutResponse, error) {
	namespace, path, ok := SplitKey(req.GetKey())
	if !ok {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"invalid key encoding: %q",
			string(req.GetKey()),
		)
	}

	if err := s.checkAccess(ctx, namespace, domain.ActionWrite); err != nil {
		return nil, err
	}

	if req.GetIgnoreValue() {
		return nil, status.Errorf(codes.Unimplemented, "ignore_value is not supported")
	}

	prev, newRev, err := s.usecase.PutKey(ctx, namespace, path, req.GetValue())
	if err != nil {
		s.recordRejectedWrite(ctx, "put", namespace, err)

		return nil, toKVStatus(err, "put", path)
	}

	resp := &etcdserverpb.PutResponse{
		Header: newHeader(newRev),
	}

	if req.GetPrevKv() && prev != nil {
		resp.PrevKv = kvPairToProto(prev)
	}

	return resp, nil
}

func (s *KVServer) DeleteRange(
	ctx context.Context,
	req *etcdserverpb.DeleteRangeRequest,
) (*etcdserverpb.DeleteRangeResponse, error) {
	startNS, startPath, endNS, endPath, ok := SplitRange(req.GetKey(), req.GetRangeEnd())
	if !ok {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"invalid key encoding: %q",
			string(req.GetKey()),
		)
	}

	if err := s.checkRangeAccess(ctx, startNS, endNS, domain.ActionWrite); err != nil {
		return nil, err
	}

	deleted, newRev, err := s.usecase.DeleteRangeKeys(
		ctx,
		startNS,
		startPath,
		endNS,
		endPath,
		req.GetPrevKv(),
	)
	if err != nil {
		s.recordRejectedWrite(ctx, "delete", startNS, err)

		return nil, toKVStatus(err, "delete range", startPath)
	}

	if newRev == 0 {
		// Nothing was deleted — return current revision.
		newRev, err = s.usecase.CurrentRevisionValue(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "get revision: %v", err)
		}
	}

	return buildDeleteRangeResponse(newRev, deleted, req.GetPrevKv()), nil
}

func (s *KVServer) Compact(
	ctx context.Context,
	_ *etcdserverpb.CompactionRequest,
) (*etcdserverpb.CompactionResponse, error) {
	// No-op — we don't truncate history.
	rev, err := s.usecase.CurrentRevisionValue(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get revision: %v", err)
	}

	return &etcdserverpb.CompactionResponse{Header: newHeader(rev)}, nil
}

func (s *KVServer) checkRangeAccess(
	ctx context.Context,
	startNS, endNS string,
	action domain.Action,
) error {
	if err := s.checkAccess(ctx, startNS, action); err != nil {
		return err
	}

	if endNS != "" && endNS != "\x00" {
		if err := s.checkAccess(ctx, endNS, action); err != nil {
			return err
		}
	}

	return nil
}

func buildDeleteRangeResponse(
	newRev int64,
	deleted []*domain.KVPair,
	prevKv bool,
) *etcdserverpb.DeleteRangeResponse {
	resp := &etcdserverpb.DeleteRangeResponse{
		Header:  newHeader(newRev),
		Deleted: int64(len(deleted)),
	}

	if prevKv {
		resp.PrevKvs = make([]*mvccpb.KeyValue, 0, len(deleted))
		for _, kv := range deleted {
			resp.PrevKvs = append(resp.PrevKvs, kvPairToProto(kv))
		}
	}

	return resp
}

func kvPairToProto(kv *domain.KVPair) *mvccpb.KeyValue {
	return &mvccpb.KeyValue{
		Key:            JoinKey(kv.Namespace, kv.Path),
		Value:          kv.Value,
		CreateRevision: kv.CreateRevision,
		ModRevision:    kv.ModRevision,
		Version:        kv.Version,
	}
}

func sortKVs(
	kvs []*mvccpb.KeyValue,
	order etcdserverpb.RangeRequest_SortOrder,
	target etcdserverpb.RangeRequest_SortTarget,
) {
	if order == etcdserverpb.RangeRequest_NONE {
		return
	}

	less := func(i, j int) bool {
		switch target {
		case etcdserverpb.RangeRequest_VERSION:
			return kvs[i].GetVersion() < kvs[j].GetVersion()
		case etcdserverpb.RangeRequest_CREATE:
			return kvs[i].GetCreateRevision() < kvs[j].GetCreateRevision()
		case etcdserverpb.RangeRequest_MOD:
			return kvs[i].GetModRevision() < kvs[j].GetModRevision()
		case etcdserverpb.RangeRequest_VALUE:
			return bytes.Compare(kvs[i].GetValue(), kvs[j].GetValue()) < 0
		default:
			return bytes.Compare(kvs[i].GetKey(), kvs[j].GetKey()) < 0
		}
	}

	if order == etcdserverpb.RangeRequest_DESCEND {
		orig := less
		less = func(i, j int) bool { return orig(j, i) }
	}

	sort.SliceStable(kvs, less)
}

func newHeader(rev int64) *etcdserverpb.ResponseHeader {
	return &etcdserverpb.ResponseHeader{
		ClusterId: clusterID,
		MemberId:  memberID,
		Revision:  rev,
		RaftTerm:  raftTerm,
	}
}

// toKVStatus maps a repo error to a gRPC status appropriate for an etcd
// client. Lock errors are normalized to a uniform "config %q is locked"
// message regardless of whether the underlying cause was a config or
// namespace lock — etcd has no concept of namespace, and clients should
// only need to react to FailedPrecondition with a path. This does NOT cover
// the full error taxonomy handler/v2/errors.go's ToConnectError maps for
// ConnectRPC (NotFound/AlreadyExists/Forbidden/VersionConflict etc.) — only
// the two kinds PutKey/DeleteRangeKeys can actually produce today. A future
// domain error introduced on this path needs its own case here; the two
// classifiers are not unified.
func toKVStatus(err error, op, path string) error {
	if errors.Is(err, domain.ErrLocked) {
		return status.Errorf(codes.FailedPrecondition, "%s: config %q is locked", op, path)
	}

	// Schema-validation rejection specifically mirrors ToConnectError's
	// CodeInvalidArgument mapping for the same domain.SchemaValidationError,
	// so a rejected write reads as "your data is invalid", not "we broke".
	if domain.IsSchemaValidationError(err) {
		return status.Errorf(codes.InvalidArgument, "%s: %v", op, err)
	}

	return status.Errorf(codes.Internal, "%s: %v", op, err)
}

func (s *KVServer) checkAccess(ctx context.Context, namespace string, action domain.Action) error {
	claims, _ := authctx.ClaimsFromContext(ctx)

	if !namespaceAllowed(claims, namespace, action) {
		return status.Errorf(codes.PermissionDenied, "permission denied for namespace %q", namespace)
	}

	return nil
}
