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
	PutKey(ctx context.Context, namespace, path string, value []byte) (*domain.Config, *domain.KVPair, int64, error)
	DeleteRangeKeys(
		ctx context.Context,
		startNS, startPath, endNS, endPath string,
		returnPrev bool,
	) ([]*domain.KVPair, int64, error)
}

// KVPublisher is the pub/sub surface for realtime events after mutations.
type KVPublisher interface {
	NotifyCreated(ctx context.Context, cfg *domain.Config)
	NotifyUpdated(ctx context.Context, cfg *domain.Config)
	NotifyDeleted(ctx context.Context, path, namespace string, revision int64)
}

// KVServer implements etcdserverpb.KVServer backed by usecase/config.
type KVServer struct {
	etcdserverpb.UnimplementedKVServer

	usecase   KVUsecase
	publisher KVPublisher
	metrics   *kvMetrics
}

func NewKVServer(usecase KVUsecase, publisher KVPublisher) *KVServer {
	return &KVServer{usecase: usecase, publisher: publisher, metrics: newKVMetrics()}
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
		Header: newHeader(currentRev),
		Kvs:    protoKVs,
		More:   more,
		Count:  count,
	}, nil
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

	cfg, prev, newRev, err := s.usecase.PutKey(ctx, namespace, path, req.GetValue())
	if err != nil {
		s.recordRejectedWrite(ctx, "put", namespace, err)

		return nil, toKVStatus(err, "put", path)
	}

	s.notifyPut(ctx, cfg, prev)

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

	if s.publisher != nil {
		for _, kv := range deleted {
			s.publisher.NotifyDeleted(ctx, kv.Path, kv.Namespace, newRev)
		}
	}

	return s.buildDeleteRangeResponse(newRev, deleted, req.GetPrevKv()), nil
}

// Txn implements a best-effort transaction. NOTE: not strictly atomic —
// compares and ops each run in their own transaction via KVUsecase (one
// evalCompare/runOp call = one usecase-level WithTx). For MVP single-instance
// this matches etcd behaviour for the common case (e.g. leader election,
// distributed locks) under low contention but can race under high write
// concurrency — confirmed empirically in
// TestIntegration_KVConformance_TxnCompareAndSwap_Concurrent
// (kv_conformance_integration_test.go), which documents this as baseline,
// not desired, behavior. Making Txn atomic requires wrapping the whole
// compare+ops sequence in one outer WithTx while deferring watch
// notification until after that commits (today's per-op notify would
// otherwise fire before a later op in the same Txn fails and rolls
// everything back) — deliberately left for a follow-up, not done here.
func (s *KVServer) Txn(
	ctx context.Context,
	req *etcdserverpb.TxnRequest,
) (*etcdserverpb.TxnResponse, error) {
	succeeded := true

	for _, cmp := range req.GetCompare() {
		ok, err := s.evalCompare(ctx, cmp)
		if err != nil {
			return nil, err
		}

		if !ok {
			succeeded = false

			break
		}
	}

	ops := req.GetSuccess()
	if !succeeded {
		ops = req.GetFailure()
	}

	responses := make([]*etcdserverpb.ResponseOp, 0, len(ops))
	var lastRev int64

	for _, op := range ops {
		resp, rev, err := s.runOp(ctx, op)
		if err != nil {
			return nil, err
		}

		if rev > lastRev {
			lastRev = rev
		}

		responses = append(responses, resp)
	}

	if lastRev == 0 {
		rev, err := s.usecase.CurrentRevisionValue(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "get revision: %v", err)
		}

		lastRev = rev
	}

	return &etcdserverpb.TxnResponse{
		Header:    newHeader(lastRev),
		Succeeded: succeeded,
		Responses: responses,
	}, nil
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

func (s *KVServer) buildDeleteRangeResponse(
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

// notifyPut fires the create/update watch notification for cfg, which
// usecase/config.Service.PutKey has already fully built (see its NOTE on
// which fields are intentionally left unpopulated).
func (s *KVServer) notifyPut(ctx context.Context, cfg *domain.Config, prev *domain.KVPair) {
	if s.publisher == nil {
		return
	}

	if prev != nil {
		s.publisher.NotifyUpdated(ctx, cfg)

		return
	}

	s.publisher.NotifyCreated(ctx, cfg)
}

func (s *KVServer) evalCompare(ctx context.Context, cmp *etcdserverpb.Compare) (bool, error) {
	startNS, startPath, endNS, endPath, ok := SplitRange(cmp.GetKey(), cmp.GetRangeEnd())
	if !ok {
		return false, status.Errorf(
			codes.InvalidArgument,
			"invalid compare key: %q",
			string(cmp.GetKey()),
		)
	}

	kvs, _, err := s.usecase.RangeKVs(ctx, startNS, startPath, endNS, endPath, configuc.KVRangeOpts{})
	if err != nil {
		return false, status.Errorf(codes.Internal, "compare range: %v", err)
	}

	// If nothing matches, all revision/version targets default to 0 and value to nil.
	if len(kvs) == 0 {
		return compareSingle(cmp, nil), nil
	}

	for _, kv := range kvs {
		if !compareSingle(cmp, kv) {
			return false, nil
		}
	}

	return true, nil
}

func compareSingle(cmp *etcdserverpb.Compare, kv *domain.KVPair) bool {
	var (
		version, createRev, modRev int64
		value                      []byte
	)

	if kv != nil {
		version = kv.Version
		createRev = kv.CreateRevision
		modRev = kv.ModRevision
		value = kv.Value
	}

	switch cmp.GetTarget() {
	case etcdserverpb.Compare_VERSION:
		want := cmp.GetVersion()

		return compareInt64(cmp.GetResult(), version, want)

	case etcdserverpb.Compare_CREATE:
		want := cmp.GetCreateRevision()

		return compareInt64(cmp.GetResult(), createRev, want)

	case etcdserverpb.Compare_MOD:
		want := cmp.GetModRevision()

		return compareInt64(cmp.GetResult(), modRev, want)

	case etcdserverpb.Compare_VALUE:
		want := cmp.GetValue()

		return compareBytes(cmp.GetResult(), value, want)

	default:
		return false
	}
}

func compareInt64(op etcdserverpb.Compare_CompareResult, got, want int64) bool {
	switch op {
	case etcdserverpb.Compare_EQUAL:
		return got == want
	case etcdserverpb.Compare_NOT_EQUAL:
		return got != want
	case etcdserverpb.Compare_GREATER:
		return got > want
	case etcdserverpb.Compare_LESS:
		return got < want
	default:
		return false
	}
}

func compareBytes(op etcdserverpb.Compare_CompareResult, got, want []byte) bool {
	cmp := bytes.Compare(got, want)

	switch op {
	case etcdserverpb.Compare_EQUAL:
		return cmp == 0
	case etcdserverpb.Compare_NOT_EQUAL:
		return cmp != 0
	case etcdserverpb.Compare_GREATER:
		return cmp > 0
	case etcdserverpb.Compare_LESS:
		return cmp < 0
	default:
		return false
	}
}

// runOp executes a single txn request op and returns its response wrapped in a ResponseOp.
// Returns the revision produced by the op (0 if op was a Range).
func (s *KVServer) runOp(
	ctx context.Context,
	op *etcdserverpb.RequestOp,
) (*etcdserverpb.ResponseOp, int64, error) {
	switch r := op.GetRequest().(type) {
	case *etcdserverpb.RequestOp_RequestRange:
		resp, err := s.Range(ctx, r.RequestRange)
		if err != nil {
			return nil, 0, err
		}

		return &etcdserverpb.ResponseOp{
			Response: &etcdserverpb.ResponseOp_ResponseRange{ResponseRange: resp},
		}, 0, nil

	case *etcdserverpb.RequestOp_RequestPut:
		resp, err := s.Put(ctx, r.RequestPut)
		if err != nil {
			return nil, 0, err
		}

		return &etcdserverpb.ResponseOp{
			Response: &etcdserverpb.ResponseOp_ResponsePut{ResponsePut: resp},
		}, resp.GetHeader().GetRevision(), nil

	case *etcdserverpb.RequestOp_RequestDeleteRange:
		resp, err := s.DeleteRange(ctx, r.RequestDeleteRange)
		if err != nil {
			return nil, 0, err
		}

		return &etcdserverpb.ResponseOp{
			Response: &etcdserverpb.ResponseOp_ResponseDeleteRange{ResponseDeleteRange: resp},
		}, resp.GetHeader().GetRevision(), nil

	case *etcdserverpb.RequestOp_RequestTxn:
		resp, err := s.Txn(ctx, r.RequestTxn)
		if err != nil {
			return nil, 0, err
		}

		return &etcdserverpb.ResponseOp{
			Response: &etcdserverpb.ResponseOp_ResponseTxn{ResponseTxn: resp},
		}, resp.GetHeader().GetRevision(), nil

	default:
		return nil, 0, status.Errorf(codes.InvalidArgument, "unknown txn request op")
	}
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
