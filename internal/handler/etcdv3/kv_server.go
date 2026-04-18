package etcdv3

import (
	"bytes"
	"context"
	"sort"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// KVRepo is the storage surface the KV server needs.
type KVRepo interface {
	CurrentRevisionValue(ctx context.Context) (int64, error)
	RangeQuery(
		ctx context.Context,
		startNS, startPath string,
		endNS, endPath string,
		limit int64,
		revision int64,
		keysOnly bool,
	) ([]*domain.KVPair, bool, error)
	PutKey(ctx context.Context, namespace, path string, value []byte) (*domain.KVPair, int64, error)
	DeleteRangeKeys(
		ctx context.Context,
		startNS, startPath string,
		endNS, endPath string,
		returnPrev bool,
	) ([]*domain.KVPair, int64, error)
}

// KVPublisher is the pub/sub surface for realtime events after mutations.
type KVPublisher interface {
	NotifyCreated(ctx context.Context, cfg *domain.Config)
	NotifyUpdated(ctx context.Context, cfg *domain.Config)
	NotifyDeleted(ctx context.Context, path, namespace string, revision int64)
}

// KVServer implements etcdserverpb.KVServer backed by bbolt storage.
type KVServer struct {
	etcdserverpb.UnimplementedKVServer

	repo      KVRepo
	publisher KVPublisher
}

func NewKVServer(repo KVRepo, publisher KVPublisher) *KVServer {
	return &KVServer{repo: repo, publisher: publisher}
}

func (s *KVServer) Range(ctx context.Context, req *etcdserverpb.RangeRequest) (*etcdserverpb.RangeResponse, error) {
	startNS, startPath, endNS, endPath, ok := SplitRange(req.Key, req.RangeEnd)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "invalid key encoding: %q", string(req.Key))
	}

	kvs, more, err := s.repo.RangeQuery(
		ctx,
		startNS, startPath,
		endNS, endPath,
		req.Limit,
		req.Revision,
		req.KeysOnly,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "range query: %v", err)
	}

	currentRev, err := s.repo.CurrentRevisionValue(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get revision: %v", err)
	}

	protoKVs := make([]*mvccpb.KeyValue, 0, len(kvs))
	for _, kv := range kvs {
		protoKVs = append(protoKVs, kvPairToProto(kv))
	}

	sortKVs(protoKVs, req.SortOrder, req.SortTarget)

	count := int64(len(protoKVs))
	if req.CountOnly {
		protoKVs = nil
	}

	return &etcdserverpb.RangeResponse{
		Header: newHeader(currentRev),
		Kvs:    protoKVs,
		More:   more,
		Count:  count,
	}, nil
}

func (s *KVServer) Put(ctx context.Context, req *etcdserverpb.PutRequest) (*etcdserverpb.PutResponse, error) {
	namespace, path, ok := SplitKey(req.Key)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "invalid key encoding: %q", string(req.Key))
	}

	if req.IgnoreValue {
		return nil, status.Errorf(codes.Unimplemented, "ignore_value is not supported")
	}

	prev, newRev, err := s.repo.PutKey(ctx, namespace, path, req.Value)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "put: %v", err)
	}

	s.notifyPut(ctx, namespace, path, req.Value, prev, newRev)

	resp := &etcdserverpb.PutResponse{
		Header: newHeader(newRev),
	}

	if req.PrevKv && prev != nil {
		resp.PrevKv = kvPairToProto(prev)
	}

	return resp, nil
}

func (s *KVServer) DeleteRange(
	ctx context.Context,
	req *etcdserverpb.DeleteRangeRequest,
) (*etcdserverpb.DeleteRangeResponse, error) {
	startNS, startPath, endNS, endPath, ok := SplitRange(req.Key, req.RangeEnd)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "invalid key encoding: %q", string(req.Key))
	}

	deleted, newRev, err := s.repo.DeleteRangeKeys(ctx, startNS, startPath, endNS, endPath, req.PrevKv)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "delete range: %v", err)
	}

	if newRev == 0 {
		// Nothing was deleted — return current revision.
		newRev, err = s.repo.CurrentRevisionValue(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "get revision: %v", err)
		}
	}

	if s.publisher != nil {
		for _, kv := range deleted {
			s.publisher.NotifyDeleted(ctx, kv.Path, kv.Namespace, newRev)
		}
	}

	resp := &etcdserverpb.DeleteRangeResponse{
		Header:  newHeader(newRev),
		Deleted: int64(len(deleted)),
	}

	if req.PrevKv {
		resp.PrevKvs = make([]*mvccpb.KeyValue, 0, len(deleted))
		for _, kv := range deleted {
			resp.PrevKvs = append(resp.PrevKvs, kvPairToProto(kv))
		}
	}

	return resp, nil
}

// Txn implements a best-effort transaction. NOTE: not strictly atomic —
// compares and ops run in separate bbolt transactions. For MVP single-instance
// this matches etcd behaviour for the common case (e.g. leader election,
// distributed locks) under low contention but can race under high write
// concurrency. TODO: expose bbolt tx to make this truly atomic.
func (s *KVServer) Txn(ctx context.Context, req *etcdserverpb.TxnRequest) (*etcdserverpb.TxnResponse, error) {
	succeeded := true

	for _, cmp := range req.Compare {
		ok, err := s.evalCompare(ctx, cmp)
		if err != nil {
			return nil, err
		}

		if !ok {
			succeeded = false

			break
		}
	}

	ops := req.Success
	if !succeeded {
		ops = req.Failure
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
		rev, err := s.repo.CurrentRevisionValue(ctx)
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
	rev, err := s.repo.CurrentRevisionValue(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get revision: %v", err)
	}

	return &etcdserverpb.CompactionResponse{Header: newHeader(rev)}, nil
}

func (s *KVServer) notifyPut(
	ctx context.Context,
	namespace, path string,
	value []byte,
	prev *domain.KVPair,
	newRev int64,
) {
	if s.publisher == nil {
		return
	}

	// NOTE: notifies with incomplete config — ContentHash, Format, Metadata,
	// and CreatedAt are not populated because PutKey returns a slim KVPair.
	// A dedicated GetKey repo method would fill all fields without a second
	// bbolt transaction.
	cfg := &domain.Config{
		Path:      path,
		Namespace: namespace,
		Content:   string(value),
		Revision:  newRev,
		UpdatedAt: time.Now(),
	}

	if prev != nil {
		cfg.Version = prev.Version + 1
		cfg.CreateRevision = prev.CreateRevision
		s.publisher.NotifyUpdated(ctx, cfg)

		return
	}

	cfg.Version = 1
	cfg.CreateRevision = newRev
	cfg.CreatedAt = cfg.UpdatedAt
	s.publisher.NotifyCreated(ctx, cfg)
}

func (s *KVServer) evalCompare(ctx context.Context, cmp *etcdserverpb.Compare) (bool, error) {
	startNS, startPath, endNS, endPath, ok := SplitRange(cmp.Key, cmp.RangeEnd)
	if !ok {
		return false, status.Errorf(codes.InvalidArgument, "invalid compare key: %q", string(cmp.Key))
	}

	kvs, _, err := s.repo.RangeQuery(ctx, startNS, startPath, endNS, endPath, 0, 0, false)
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

	switch cmp.Target {
	case etcdserverpb.Compare_VERSION:
		want := cmp.GetVersion()

		return compareInt64(cmp.Result, version, want)

	case etcdserverpb.Compare_CREATE:
		want := cmp.GetCreateRevision()

		return compareInt64(cmp.Result, createRev, want)

	case etcdserverpb.Compare_MOD:
		want := cmp.GetModRevision()

		return compareInt64(cmp.Result, modRev, want)

	case etcdserverpb.Compare_VALUE:
		want := cmp.GetValue()

		return compareBytes(cmp.Result, value, want)

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
func (s *KVServer) runOp(ctx context.Context, op *etcdserverpb.RequestOp) (*etcdserverpb.ResponseOp, int64, error) {
	switch r := op.Request.(type) {
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
		}, resp.Header.Revision, nil

	case *etcdserverpb.RequestOp_RequestDeleteRange:
		resp, err := s.DeleteRange(ctx, r.RequestDeleteRange)
		if err != nil {
			return nil, 0, err
		}

		return &etcdserverpb.ResponseOp{
			Response: &etcdserverpb.ResponseOp_ResponseDeleteRange{ResponseDeleteRange: resp},
		}, resp.Header.Revision, nil

	case *etcdserverpb.RequestOp_RequestTxn:
		resp, err := s.Txn(ctx, r.RequestTxn)
		if err != nil {
			return nil, 0, err
		}

		return &etcdserverpb.ResponseOp{
			Response: &etcdserverpb.ResponseOp_ResponseTxn{ResponseTxn: resp},
		}, resp.Header.Revision, nil

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
			return kvs[i].Version < kvs[j].Version
		case etcdserverpb.RangeRequest_CREATE:
			return kvs[i].CreateRevision < kvs[j].CreateRevision
		case etcdserverpb.RangeRequest_MOD:
			return kvs[i].ModRevision < kvs[j].ModRevision
		case etcdserverpb.RangeRequest_VALUE:
			return bytes.Compare(kvs[i].Value, kvs[j].Value) < 0
		default:
			return bytes.Compare(kvs[i].Key, kvs[j].Key) < 0
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
