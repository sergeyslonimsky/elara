package etcdv3_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/etcdv3"
	configuc "github.com/sergeyslonimsky/elara/internal/usecase/config"
)

// fakeKVUsecase stands in for usecase/config.Service — the KVUsecase surface,
// not a repository, despite what the KV path's storage-shaped method names
// suggest.
//
// Range/Put/DeleteRange are backed by a real in-memory store so the handler's
// wire shaping can be asserted end-to-end. Txn deliberately is NOT: evaluating
// the guard, choosing a branch and ordering notifications against the commit
// live in the usecase and are tested there
// (usecase/config/service_txn_test.go). Re-implementing that here would assert
// the fake rather than the handler, so Txn only records the converted input and
// replays a scripted result — which is exactly the handler's own contract:
// proto→DTO conversion, authorization, DTO→proto conversion.
type fakeKVUsecase struct {
	mu    sync.Mutex
	pairs map[string]*domain.KVPair // key: ns+"\x00"+path
	rev   int64

	// injection hooks for error paths
	rangeErr   error
	putErr     error
	deleteErr  error
	currentErr error

	// Txn recorder/replayer.
	txnCalls  []configuc.KVTxnInput
	txnResult configuc.KVTxnResult
	txnErr    error
}

func newFakeKVUsecase() *fakeKVUsecase {
	return &fakeKVUsecase{pairs: make(map[string]*domain.KVPair)}
}

func (f *fakeKVUsecase) CurrentRevisionValue(_ context.Context) (int64, error) {
	if f.currentErr != nil {
		return 0, f.currentErr
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	return f.rev, nil
}

func (f *fakeKVUsecase) RangeKVs(
	_ context.Context,
	startNS, startPath, endNS, endPath string,
	opts configuc.KVRangeOpts,
) ([]*domain.KVPair, bool, error) {
	if f.rangeErr != nil {
		return nil, false, f.rangeErr
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	var results []*domain.KVPair

	for _, k := range f.matchKeys(startNS, startPath, endNS, endPath) {
		clone := *f.pairs[k]
		if opts.KeysOnly {
			clone.Value = nil
		}

		results = append(results, &clone)
	}

	sortByKey(results)

	more := false
	if opts.Limit > 0 && int64(len(results)) > opts.Limit {
		results = results[:opts.Limit]
		more = true
	}

	return results, more, nil
}

func (f *fakeKVUsecase) RangeQuery(
	ctx context.Context,
	startNS, startPath, endNS, endPath string,
	opts configuc.KVRangeOpts,
) ([]*domain.KVPair, int64, bool, error) {
	results, more, err := f.RangeKVs(ctx, startNS, startPath, endNS, endPath, opts)
	if err != nil {
		return nil, 0, false, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.currentErr != nil {
		return nil, 0, false, f.currentErr
	}

	return results, f.rev, more, nil
}

func (f *fakeKVUsecase) PutKey(
	_ context.Context,
	namespace, path string,
	value []byte,
) (*domain.KVPair, int64, error) {
	if f.putErr != nil {
		return nil, 0, f.putErr
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	k := f.key(namespace, path)
	prev := f.pairs[k]

	f.rev++

	valCopy := make([]byte, len(value))
	copy(valCopy, value)

	if prev == nil {
		f.pairs[k] = &domain.KVPair{
			Namespace:      namespace,
			Path:           path,
			Value:          valCopy,
			CreateRevision: f.rev,
			ModRevision:    f.rev,
			Version:        1,
		}

		return nil, f.rev, nil
	}

	// Return a copy so callers cannot mutate our internal state.
	prevCopy := *prev
	f.pairs[k] = &domain.KVPair{
		Namespace:      namespace,
		Path:           path,
		Value:          valCopy,
		CreateRevision: prev.CreateRevision,
		ModRevision:    f.rev,
		Version:        prev.Version + 1,
	}

	return &prevCopy, f.rev, nil
}

func (f *fakeKVUsecase) DeleteRangeKeys(
	_ context.Context,
	startNS, startPath string,
	endNS, endPath string,
	returnPrev bool,
) ([]*domain.KVPair, int64, error) {
	if f.deleteErr != nil {
		return nil, 0, f.deleteErr
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	keysToDelete := f.matchKeys(startNS, startPath, endNS, endPath)

	if len(keysToDelete) == 0 {
		return nil, 0, nil
	}

	f.rev++

	var deleted []*domain.KVPair

	for _, k := range keysToDelete {
		kv := f.pairs[k]
		delete(f.pairs, k)

		if returnPrev {
			kvCopy := *kv
			deleted = append(deleted, &kvCopy)
		} else {
			deleted = append(deleted, &domain.KVPair{Namespace: kv.Namespace, Path: kv.Path})
		}
	}

	sortByKey(deleted)

	return deleted, f.rev, nil
}

func (f *fakeKVUsecase) Txn(
	_ context.Context,
	in configuc.KVTxnInput,
) (configuc.KVTxnResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.txnCalls = append(f.txnCalls, in)

	if f.txnErr != nil {
		return configuc.KVTxnResult{}, f.txnErr
	}

	return f.txnResult, nil
}

// lastTxnInput returns the single KVTxnInput the handler converted, failing the
// test if the usecase was not called exactly once.
func (f *fakeKVUsecase) lastTxnInput(t *testing.T) configuc.KVTxnInput {
	t.Helper()

	f.mu.Lock()
	defer f.mu.Unlock()

	require.Len(t, f.txnCalls, 1, "usecase.Txn called exactly once")

	return f.txnCalls[0]
}

func (f *fakeKVUsecase) txnCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.txnCalls)
}

func (f *fakeKVUsecase) key(ns, path string) string { return ns + "\x00" + path }

// matchKeys returns the sorted list of map keys that fall within the given
// etcd-style [start, end) range. Centralises the range-match logic so
// RangeQuery and DeleteRangeKeys don't each re-implement it.
func (f *fakeKVUsecase) matchKeys(startNS, startPath, endNS, endPath string) []string {
	single := endNS == "" && endPath == ""
	scanAll := endNS == "\x00"
	startKey := f.key(startNS, startPath)
	endKey := f.key(endNS, endPath)

	keys := make([]string, 0, len(f.pairs))

	for k := range f.pairs {
		switch {
		case single:
			if k != startKey {
				continue
			}
		case scanAll:
			if k < startKey {
				continue
			}
		default:
			if k < startKey || k >= endKey {
				continue
			}
		}

		keys = append(keys, k)
	}

	return keys
}

func sortByKey(kvs []*domain.KVPair) {
	for i := 1; i < len(kvs); i++ {
		for j := i; j > 0; j-- {
			if kvs[j].Namespace+"\x00"+kvs[j].Path < kvs[j-1].Namespace+"\x00"+kvs[j-1].Path {
				kvs[j], kvs[j-1] = kvs[j-1], kvs[j]
			} else {
				break
			}
		}
	}
}

// -----------------------------------------------------------------------------
// Put / Range / DeleteRange / Compact
// -----------------------------------------------------------------------------

func TestKVServer_Put_CreatesNewKey(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVUsecase())

	resp, err := s.Put(t.Context(), &etcdserverpb.PutRequest{
		Key:   []byte("/default/foo.json"),
		Value: []byte("hello"),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.GetHeader().GetRevision())
	assert.Nil(t, resp.GetPrevKv())
}

func TestKVServer_Put_UpdatesExistingKey_WithPrevKv(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVUsecase())
	ctx := t.Context()

	_, err := s.Put(ctx, &etcdserverpb.PutRequest{
		Key: []byte("/default/foo"), Value: []byte("v1"),
	})
	require.NoError(t, err)

	resp, err := s.Put(ctx, &etcdserverpb.PutRequest{
		Key: []byte("/default/foo"), Value: []byte("v2"), PrevKv: true,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.GetHeader().GetRevision())
	require.NotNil(t, resp.GetPrevKv())
	assert.Equal(t, []byte("v1"), resp.GetPrevKv().GetValue())
	assert.Equal(t, int64(1), resp.GetPrevKv().GetVersion())
}

func TestKVServer_Put_InvalidKey(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVUsecase())

	_, err := s.Put(t.Context(), &etcdserverpb.PutRequest{Key: []byte("bad")})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestKVServer_Put_IgnoreValueUnsupported(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVUsecase())

	_, err := s.Put(t.Context(), &etcdserverpb.PutRequest{
		Key: []byte("/ns/x"), IgnoreValue: true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Unimplemented, status.Code(err))
}

func TestKVServer_Put_UsecaseError(t *testing.T) {
	t.Parallel()

	uc := newFakeKVUsecase()
	uc.putErr = errors.New("boom")
	s := etcdv3.NewKVServer(uc)

	_, err := s.Put(t.Context(), &etcdserverpb.PutRequest{
		Key: []byte("/ns/x"), Value: []byte("v"),
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestKVServer_Range_SingleKey(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVUsecase())
	ctx := t.Context()

	_, err := s.Put(ctx, &etcdserverpb.PutRequest{
		Key: []byte("/default/foo"), Value: []byte("v1"),
	})
	require.NoError(t, err)

	resp, err := s.Range(ctx, &etcdserverpb.RangeRequest{
		Key: []byte("/default/foo"),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.GetCount())
	require.Len(t, resp.GetKvs(), 1)
	assert.Equal(t, []byte("/default/foo"), resp.GetKvs()[0].GetKey())
	assert.Equal(t, []byte("v1"), resp.GetKvs()[0].GetValue())
}

func TestKVServer_Range_Prefix(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVUsecase())
	ctx := t.Context()

	for _, k := range []string{"/default/a", "/default/b", "/default/c", "/prod/x"} {
		_, err := s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte(k), Value: []byte("v")})
		require.NoError(t, err)
	}

	// Prefix /default/ → rangeEnd increments last byte
	resp, err := s.Range(ctx, &etcdserverpb.RangeRequest{
		Key:      []byte("/default/"),
		RangeEnd: []byte("/default0"),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(3), resp.GetCount())
	assert.Len(t, resp.GetKvs(), 3)
}

func TestKVServer_Range_Limit_ReportsMore(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVUsecase())
	ctx := t.Context()

	for _, k := range []string{"/ns/a", "/ns/b", "/ns/c"} {
		_, err := s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte(k), Value: []byte("v")})
		require.NoError(t, err)
	}

	resp, err := s.Range(ctx, &etcdserverpb.RangeRequest{
		Key:      []byte("/ns/"),
		RangeEnd: []byte("/ns0"),
		Limit:    2,
	})
	require.NoError(t, err)
	assert.Len(t, resp.GetKvs(), 2)
	assert.True(t, resp.GetMore(), "More should be true when results truncated")
}

func TestKVServer_Range_CountOnly(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVUsecase())
	ctx := t.Context()

	for _, k := range []string{"/ns/a", "/ns/b"} {
		_, err := s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte(k), Value: []byte("v")})
		require.NoError(t, err)
	}

	resp, err := s.Range(ctx, &etcdserverpb.RangeRequest{
		Key: []byte("/ns/"), RangeEnd: []byte("/ns0"), CountOnly: true,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.GetCount())
	assert.Empty(t, resp.GetKvs())
}

func TestKVServer_Range_KeysOnly(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVUsecase())
	ctx := t.Context()

	_, err := s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte("/ns/a"), Value: []byte("v1")})
	require.NoError(t, err)

	resp, err := s.Range(ctx, &etcdserverpb.RangeRequest{
		Key: []byte("/ns/a"), KeysOnly: true,
	})
	require.NoError(t, err)
	require.Len(t, resp.GetKvs(), 1)
	assert.Empty(t, resp.GetKvs()[0].GetValue(), "KeysOnly must strip Value")
}

func TestKVServer_Range_Sort_Descend(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVUsecase())
	ctx := t.Context()

	for _, k := range []string{"/ns/a", "/ns/b", "/ns/c"} {
		_, err := s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte(k), Value: []byte("v")})
		require.NoError(t, err)
	}

	resp, err := s.Range(ctx, &etcdserverpb.RangeRequest{
		Key: []byte("/ns/"), RangeEnd: []byte("/ns0"),
		SortOrder: etcdserverpb.RangeRequest_DESCEND, SortTarget: etcdserverpb.RangeRequest_KEY,
	})
	require.NoError(t, err)
	require.Len(t, resp.GetKvs(), 3)
	assert.Equal(t, []byte("/ns/c"), resp.GetKvs()[0].GetKey())
	assert.Equal(t, []byte("/ns/a"), resp.GetKvs()[2].GetKey())
}

func TestKVServer_Range_InvalidKey(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVUsecase())

	_, err := s.Range(t.Context(), &etcdserverpb.RangeRequest{Key: []byte("bad")})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestKVServer_Range_UsecaseError(t *testing.T) {
	t.Parallel()

	uc := newFakeKVUsecase()
	uc.rangeErr = errors.New("boom")
	s := etcdv3.NewKVServer(uc)

	_, err := s.Range(t.Context(), &etcdserverpb.RangeRequest{Key: []byte("/ns/x")})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "boom")
}

func TestKVServer_DeleteRange_SingleKey(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVUsecase())
	ctx := t.Context()

	_, err := s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte("/ns/x"), Value: []byte("v")})
	require.NoError(t, err)

	resp, err := s.DeleteRange(ctx, &etcdserverpb.DeleteRangeRequest{
		Key: []byte("/ns/x"), PrevKv: true,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.GetDeleted())
	require.Len(t, resp.GetPrevKvs(), 1)
	assert.Equal(t, []byte("v"), resp.GetPrevKvs()[0].GetValue())
	assert.Equal(t, int64(2), resp.GetHeader().GetRevision())
}

func TestKVServer_DeleteRange_Nothing_ReturnsCurrentRev(t *testing.T) {
	t.Parallel()

	uc := newFakeKVUsecase()
	uc.rev = 7
	s := etcdv3.NewKVServer(uc)

	resp, err := s.DeleteRange(t.Context(), &etcdserverpb.DeleteRangeRequest{
		Key: []byte("/ns/missing"),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), resp.GetDeleted())
	assert.Equal(t, int64(7), resp.GetHeader().GetRevision())
}

func TestKVServer_DeleteRange_Prefix(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVUsecase())
	ctx := t.Context()

	for _, k := range []string{"/ns/a", "/ns/b", "/other/c"} {
		_, err := s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte(k), Value: []byte("v")})
		require.NoError(t, err)
	}

	resp, err := s.DeleteRange(ctx, &etcdserverpb.DeleteRangeRequest{
		Key: []byte("/ns/"), RangeEnd: []byte("/ns0"),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.GetDeleted())
}

func TestKVServer_DeleteRange_InvalidKey(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVUsecase())

	_, err := s.DeleteRange(
		t.Context(),
		&etcdserverpb.DeleteRangeRequest{Key: []byte("bad")},
	)
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestKVServer_DeleteRange_CurrentRevError(t *testing.T) {
	t.Parallel()

	// When no keys match AND CurrentRevisionValue errors, DeleteRange returns Internal.
	uc := newFakeKVUsecase()
	uc.currentErr = errors.New("boom")
	s := etcdv3.NewKVServer(uc)

	_, err := s.DeleteRange(
		t.Context(),
		&etcdserverpb.DeleteRangeRequest{Key: []byte("/ns/x")},
	)
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestKVServer_DeleteRange_UsecaseError(t *testing.T) {
	t.Parallel()

	uc := newFakeKVUsecase()
	uc.deleteErr = errors.New("boom")
	s := etcdv3.NewKVServer(uc)

	_, err := s.DeleteRange(
		t.Context(),
		&etcdserverpb.DeleteRangeRequest{Key: []byte("/ns/x")},
	)
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestKVServer_Range_CurrentRevError(t *testing.T) {
	t.Parallel()

	uc := newFakeKVUsecase()
	uc.currentErr = errors.New("boom")
	s := etcdv3.NewKVServer(uc)

	_, err := s.Range(t.Context(), &etcdserverpb.RangeRequest{Key: []byte("/ns/x")})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestKVServer_Compact_IsNoOp(t *testing.T) {
	t.Parallel()

	uc := newFakeKVUsecase()
	uc.rev = 42
	s := etcdv3.NewKVServer(uc)

	resp, err := s.Compact(t.Context(), &etcdserverpb.CompactionRequest{Revision: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(42), resp.GetHeader().GetRevision())
}

func TestKVServer_Compact_CurrentRevError(t *testing.T) {
	t.Parallel()

	uc := newFakeKVUsecase()
	uc.currentErr = errors.New("boom")
	s := etcdv3.NewKVServer(uc)

	_, err := s.Compact(t.Context(), &etcdserverpb.CompactionRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// -----------------------------------------------------------------------------
// Txn — proto→DTO conversion
// -----------------------------------------------------------------------------

func TestKVServer_Txn_ConvertsRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  *etcdserverpb.TxnRequest
		want configuc.KVTxnInput
	}{
		{
			name: "create-if-absent compare",
			req: &etcdserverpb.TxnRequest{
				Compare: []*etcdserverpb.Compare{{
					Key:         []byte("/ns/k"),
					Target:      etcdserverpb.Compare_CREATE,
					Result:      etcdserverpb.Compare_EQUAL,
					TargetUnion: &etcdserverpb.Compare_CreateRevision{CreateRevision: 0},
				}},
			},
			want: configuc.KVTxnInput{
				Compare: []configuc.KVCompare{{
					StartNS:      "ns",
					StartPath:    "/k",
					Target:       configuc.KVCompareCreateRevision,
					Result:       configuc.KVCompareEqual,
					WantRevision: 0,
				}},
				Success: []configuc.KVOp{},
				Failure: []configuc.KVOp{},
			},
		},
		{
			name: "mod-revision compare carries the revision, not the value",
			req: &etcdserverpb.TxnRequest{
				Compare: []*etcdserverpb.Compare{{
					Key:         []byte("/ns/k"),
					Target:      etcdserverpb.Compare_MOD,
					Result:      etcdserverpb.Compare_GREATER,
					TargetUnion: &etcdserverpb.Compare_ModRevision{ModRevision: 12},
				}},
			},
			want: configuc.KVTxnInput{
				Compare: []configuc.KVCompare{{
					StartNS:      "ns",
					StartPath:    "/k",
					Target:       configuc.KVCompareModRevision,
					Result:       configuc.KVCompareGreater,
					WantRevision: 12,
				}},
				Success: []configuc.KVOp{},
				Failure: []configuc.KVOp{},
			},
		},
		{
			name: "version compare over a range",
			req: &etcdserverpb.TxnRequest{
				Compare: []*etcdserverpb.Compare{{
					Key:         []byte("/ns/"),
					RangeEnd:    []byte("/ns0"),
					Target:      etcdserverpb.Compare_VERSION,
					Result:      etcdserverpb.Compare_LESS,
					TargetUnion: &etcdserverpb.Compare_Version{Version: 3},
				}},
			},
			want: configuc.KVTxnInput{
				Compare: []configuc.KVCompare{{
					StartNS:      "ns",
					StartPath:    "/",
					EndNS:        "ns0",
					EndPath:      "/",
					Target:       configuc.KVCompareVersion,
					Result:       configuc.KVCompareLess,
					WantRevision: 3,
				}},
				Success: []configuc.KVOp{},
				Failure: []configuc.KVOp{},
			},
		},
		{
			name: "value compare carries the value, not a revision",
			req: &etcdserverpb.TxnRequest{
				Compare: []*etcdserverpb.Compare{{
					Key:         []byte("/ns/k"),
					Target:      etcdserverpb.Compare_VALUE,
					Result:      etcdserverpb.Compare_NOT_EQUAL,
					TargetUnion: &etcdserverpb.Compare_Value{Value: []byte("held")},
				}},
			},
			want: configuc.KVTxnInput{
				Compare: []configuc.KVCompare{{
					StartNS:   "ns",
					StartPath: "/k",
					Target:    configuc.KVCompareValue,
					Result:    configuc.KVCompareNotEqual,
					WantValue: []byte("held"),
				}},
				Success: []configuc.KVOp{},
				Failure: []configuc.KVOp{},
			},
		},
		{
			// LEASE is a target this server does not implement. It must not be
			// rejected: the usecase treats target 0 as a condition that does not
			// hold, so an odd compare selects the failure branch — the behaviour
			// this path had before the orchestration moved.
			name: "unsupported compare target maps to zero, not an error",
			req: &etcdserverpb.TxnRequest{
				Compare: []*etcdserverpb.Compare{{
					Key:    []byte("/ns/k"),
					Target: etcdserverpb.Compare_LEASE,
					Result: etcdserverpb.Compare_EQUAL,
				}},
			},
			want: configuc.KVTxnInput{
				Compare: []configuc.KVCompare{{
					StartNS:   "ns",
					StartPath: "/k",
					Target:    0,
					Result:    configuc.KVCompareEqual,
				}},
				Success: []configuc.KVOp{},
				Failure: []configuc.KVOp{},
			},
		},
		{
			name: "unrecognised compare target and result both map to zero",
			req: &etcdserverpb.TxnRequest{
				Compare: []*etcdserverpb.Compare{{
					Key:    []byte("/ns/k"),
					Target: etcdserverpb.Compare_CompareTarget(99),
					Result: etcdserverpb.Compare_CompareResult(99),
				}},
			},
			want: configuc.KVTxnInput{
				Compare: []configuc.KVCompare{{
					StartNS:   "ns",
					StartPath: "/k",
					Target:    0,
					Result:    0,
				}},
				Success: []configuc.KVOp{},
				Failure: []configuc.KVOp{},
			},
		},
		{
			name: "put op splits the key into namespace and path",
			req: &etcdserverpb.TxnRequest{
				Success: []*etcdserverpb.RequestOp{{
					Request: &etcdserverpb.RequestOp_RequestPut{
						RequestPut: &etcdserverpb.PutRequest{
							Key:   []byte("/prod/svc/api.yaml"),
							Value: []byte("ok"),
						},
					},
				}},
			},
			want: configuc.KVTxnInput{
				Compare: []configuc.KVCompare{},
				Success: []configuc.KVOp{{
					Kind: configuc.KVOpPut,
					Put: &configuc.KVPutOp{
						Namespace: "prod",
						Path:      "/svc/api.yaml",
						Value:     []byte("ok"),
					},
				}},
				Failure: []configuc.KVOp{},
			},
		},
		{
			name: "range op carries the range options",
			req: &etcdserverpb.TxnRequest{
				Success: []*etcdserverpb.RequestOp{{
					Request: &etcdserverpb.RequestOp_RequestRange{
						RequestRange: &etcdserverpb.RangeRequest{
							Key:      []byte("/ns/a"),
							Limit:    5,
							Revision: 9,
							KeysOnly: true,
						},
					},
				}},
			},
			want: configuc.KVTxnInput{
				Compare: []configuc.KVCompare{},
				Success: []configuc.KVOp{{
					Kind: configuc.KVOpRange,
					Range: &configuc.KVRangeOp{
						StartNS:   "ns",
						StartPath: "/a",
						Opts: configuc.KVRangeOpts{
							Limit:    5,
							Revision: 9,
							KeysOnly: true,
						},
					},
				}},
				Failure: []configuc.KVOp{},
			},
		},
		{
			name: "delete-range op in the failure branch carries prev_kv",
			req: &etcdserverpb.TxnRequest{
				Failure: []*etcdserverpb.RequestOp{{
					Request: &etcdserverpb.RequestOp_RequestDeleteRange{
						RequestDeleteRange: &etcdserverpb.DeleteRangeRequest{
							Key:      []byte("/ns/"),
							RangeEnd: []byte{0},
							PrevKv:   true,
						},
					},
				}},
			},
			want: configuc.KVTxnInput{
				Compare: []configuc.KVCompare{},
				Success: []configuc.KVOp{},
				Failure: []configuc.KVOp{{
					Kind: configuc.KVOpDeleteRange,
					DeleteRange: &configuc.KVDeleteRangeOp{
						StartNS:    "ns",
						StartPath:  "/",
						EndNS:      "\x00",
						ReturnPrev: true,
					},
				}},
			},
		},
		{
			name: "nested txn converts recursively",
			req: &etcdserverpb.TxnRequest{
				Success: []*etcdserverpb.RequestOp{{
					Request: &etcdserverpb.RequestOp_RequestTxn{
						RequestTxn: &etcdserverpb.TxnRequest{
							Success: []*etcdserverpb.RequestOp{{
								Request: &etcdserverpb.RequestOp_RequestPut{
									RequestPut: &etcdserverpb.PutRequest{
										Key:   []byte("/ns/nested"),
										Value: []byte("v"),
									},
								},
							}},
						},
					},
				}},
			},
			want: configuc.KVTxnInput{
				Compare: []configuc.KVCompare{},
				Success: []configuc.KVOp{{
					Kind: configuc.KVOpTxn,
					Txn: &configuc.KVTxnInput{
						Compare: []configuc.KVCompare{},
						Success: []configuc.KVOp{{
							Kind: configuc.KVOpPut,
							Put: &configuc.KVPutOp{
								Namespace: "ns",
								Path:      "/nested",
								Value:     []byte("v"),
							},
						}},
						Failure: []configuc.KVOp{},
					},
				}},
				Failure: []configuc.KVOp{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := newFakeKVUsecase()
			// The branch that runs must have as many results as it has
			// requests; every case above has at most one op per branch.
			uc.txnResult = configuc.KVTxnResult{
				Succeeded: true,
				Revision:  1,
				Responses: txnResultsFor(tt.req.GetSuccess()),
			}
			s := etcdv3.NewKVServer(uc)

			_, err := s.Txn(t.Context(), tt.req)
			require.NoError(t, err)

			assert.Equal(t, tt.want, uc.lastTxnInput(t))
		})
	}
}

// txnResultsFor builds the minimum set of results the response converter needs
// for the given requests — one per op, with the matching pointer populated.
func txnResultsFor(ops []*etcdserverpb.RequestOp) []configuc.KVOpResult {
	results := make([]configuc.KVOpResult, 0, len(ops))

	for _, op := range ops {
		switch op.GetRequest().(type) {
		case *etcdserverpb.RequestOp_RequestRange:
			results = append(results, configuc.KVOpResult{
				Kind:  configuc.KVOpRange,
				Range: &configuc.KVRangeResult{},
			})
		case *etcdserverpb.RequestOp_RequestPut:
			results = append(results, configuc.KVOpResult{
				Kind: configuc.KVOpPut,
				Put:  &configuc.KVPutResult{},
			})
		case *etcdserverpb.RequestOp_RequestDeleteRange:
			results = append(results, configuc.KVOpResult{
				Kind:        configuc.KVOpDeleteRange,
				DeleteRange: &configuc.KVDeleteRangeResult{},
			})
		case *etcdserverpb.RequestOp_RequestTxn:
			results = append(results, configuc.KVOpResult{
				Kind: configuc.KVOpTxn,
				Txn:  &configuc.KVTxnResult{Responses: []configuc.KVOpResult{}},
			})
		}
	}

	return results
}

func TestKVServer_Txn_RejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		req      *etcdserverpb.TxnRequest
		wantCode codes.Code
	}{
		{
			name: "compare key is not /namespace/path",
			req: &etcdserverpb.TxnRequest{
				Compare: []*etcdserverpb.Compare{{Key: []byte("bad")}},
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "compare range end is not /namespace/path",
			req: &etcdserverpb.TxnRequest{
				Compare: []*etcdserverpb.Compare{{
					Key: []byte("/ns/a"), RangeEnd: []byte("bad"),
				}},
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "put key is not /namespace/path",
			req: &etcdserverpb.TxnRequest{
				Success: []*etcdserverpb.RequestOp{{
					Request: &etcdserverpb.RequestOp_RequestPut{
						RequestPut: &etcdserverpb.PutRequest{Key: []byte("bad")},
					},
				}},
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "range key is not /namespace/path",
			req: &etcdserverpb.TxnRequest{
				Success: []*etcdserverpb.RequestOp{{
					Request: &etcdserverpb.RequestOp_RequestRange{
						RequestRange: &etcdserverpb.RangeRequest{Key: []byte("bad")},
					},
				}},
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "delete-range key is not /namespace/path",
			req: &etcdserverpb.TxnRequest{
				Failure: []*etcdserverpb.RequestOp{{
					Request: &etcdserverpb.RequestOp_RequestDeleteRange{
						RequestDeleteRange: &etcdserverpb.DeleteRangeRequest{Key: []byte("bad")},
					},
				}},
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "op with no request set",
			req: &etcdserverpb.TxnRequest{
				Success: []*etcdserverpb.RequestOp{{Request: nil}},
			},
			wantCode: codes.InvalidArgument,
		},
		{
			// Rejected during conversion, exactly as a standalone Put is.
			name: "ignore_value inside a txn put",
			req: &etcdserverpb.TxnRequest{
				Success: []*etcdserverpb.RequestOp{{
					Request: &etcdserverpb.RequestOp_RequestPut{
						RequestPut: &etcdserverpb.PutRequest{
							Key: []byte("/ns/x"), IgnoreValue: true,
						},
					},
				}},
			},
			wantCode: codes.Unimplemented,
		},
		{
			name: "invalid op nested inside a nested txn",
			req: &etcdserverpb.TxnRequest{
				Success: []*etcdserverpb.RequestOp{{
					Request: &etcdserverpb.RequestOp_RequestTxn{
						RequestTxn: &etcdserverpb.TxnRequest{
							Success: []*etcdserverpb.RequestOp{{
								Request: &etcdserverpb.RequestOp_RequestPut{
									RequestPut: &etcdserverpb.PutRequest{Key: []byte("bad")},
								},
							}},
						},
					},
				}},
			},
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := newFakeKVUsecase()
			s := etcdv3.NewKVServer(uc)

			_, err := s.Txn(t.Context(), tt.req)
			require.Error(t, err)
			assert.Equal(t, tt.wantCode, status.Code(err))
			assert.Equal(t, 0, uc.txnCallCount(), "a rejected request never reaches the usecase")
		})
	}
}

// -----------------------------------------------------------------------------
// Txn — authorization
// -----------------------------------------------------------------------------

func TestKVServer_Txn_PermissionDenied(t *testing.T) {
	t.Parallel()

	writerOnAllowed := &authctx.Claims{Namespaces: []string{"allowed"}, Role: "writer"}

	holdingCompare := []*etcdserverpb.Compare{{
		Key:         []byte("/allowed/k"),
		Target:      etcdserverpb.Compare_CREATE,
		Result:      etcdserverpb.Compare_EQUAL,
		TargetUnion: &etcdserverpb.Compare_CreateRevision{CreateRevision: 0},
	}}

	putOp := func(key string) *etcdserverpb.RequestOp {
		return &etcdserverpb.RequestOp{
			Request: &etcdserverpb.RequestOp_RequestPut{
				RequestPut: &etcdserverpb.PutRequest{Key: []byte(key), Value: []byte("v")},
			},
		}
	}

	tests := []struct {
		name   string
		claims *authctx.Claims
		req    *etcdserverpb.TxnRequest
	}{
		{
			// The branch is not known until the compares run inside the
			// transaction, so both are authorized up front — as etcd does. A
			// forbidden write in the branch that would NOT have run still
			// rejects the request.
			name:   "failure branch writes to a forbidden namespace",
			claims: writerOnAllowed,
			req: &etcdserverpb.TxnRequest{
				Compare: holdingCompare,
				Success: []*etcdserverpb.RequestOp{putOp("/allowed/k")},
				Failure: []*etcdserverpb.RequestOp{putOp("/denied/k")},
			},
		},
		{
			name:   "success branch writes to a forbidden namespace",
			claims: writerOnAllowed,
			req: &etcdserverpb.TxnRequest{
				Compare: holdingCompare,
				Success: []*etcdserverpb.RequestOp{putOp("/denied/k")},
			},
		},
		{
			name:   "compare reads a forbidden namespace",
			claims: writerOnAllowed,
			req: &etcdserverpb.TxnRequest{
				Compare: []*etcdserverpb.Compare{{
					Key:    []byte("/denied/k"),
					Target: etcdserverpb.Compare_CREATE,
					Result: etcdserverpb.Compare_EQUAL,
				}},
			},
		},
		{
			name:   "range op reads a forbidden namespace",
			claims: writerOnAllowed,
			req: &etcdserverpb.TxnRequest{
				Success: []*etcdserverpb.RequestOp{{
					Request: &etcdserverpb.RequestOp_RequestRange{
						RequestRange: &etcdserverpb.RangeRequest{Key: []byte("/denied/k")},
					},
				}},
			},
		},
		{
			name:   "delete-range op writes to a forbidden namespace",
			claims: writerOnAllowed,
			req: &etcdserverpb.TxnRequest{
				Failure: []*etcdserverpb.RequestOp{{
					Request: &etcdserverpb.RequestOp_RequestDeleteRange{
						RequestDeleteRange: &etcdserverpb.DeleteRangeRequest{
							Key: []byte("/denied/k"),
						},
					},
				}},
			},
		},
		{
			name:   "nested txn writes to a forbidden namespace",
			claims: writerOnAllowed,
			req: &etcdserverpb.TxnRequest{
				Success: []*etcdserverpb.RequestOp{{
					Request: &etcdserverpb.RequestOp_RequestTxn{
						RequestTxn: &etcdserverpb.TxnRequest{
							Success: []*etcdserverpb.RequestOp{putOp("/denied/k")},
						},
					},
				}},
			},
		},
		{
			name:   "reader role cannot write to a namespace it can read",
			claims: &authctx.Claims{Namespaces: []string{"allowed"}, Role: "reader"},
			req: &etcdserverpb.TxnRequest{
				Compare: holdingCompare,
				Success: []*etcdserverpb.RequestOp{putOp("/allowed/k")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := newFakeKVUsecase()
			s := etcdv3.NewKVServer(uc)

			ctx := authctx.WithClaims(t.Context(), tt.claims)

			_, err := s.Txn(ctx, tt.req)
			require.Error(t, err)
			assert.Equal(t, codes.PermissionDenied, status.Code(err))
			assert.Equal(t, 0, uc.txnCallCount(), "a denied request never reaches the usecase")
		})
	}
}

func TestKVServer_Txn_AuthorizedWriteReachesUsecase(t *testing.T) {
	t.Parallel()

	uc := newFakeKVUsecase()
	uc.txnResult = configuc.KVTxnResult{
		Succeeded: true,
		Revision:  4,
		Responses: []configuc.KVOpResult{{
			Kind: configuc.KVOpPut,
			Put:  &configuc.KVPutResult{Revision: 4},
		}},
	}
	s := etcdv3.NewKVServer(uc)

	ctx := authctx.WithClaims(
		t.Context(),
		&authctx.Claims{Namespaces: []string{"allowed"}, Role: "writer"},
	)

	resp, err := s.Txn(ctx, &etcdserverpb.TxnRequest{
		Compare: []*etcdserverpb.Compare{{
			Key:         []byte("/allowed/k"),
			Target:      etcdserverpb.Compare_CREATE,
			Result:      etcdserverpb.Compare_EQUAL,
			TargetUnion: &etcdserverpb.Compare_CreateRevision{CreateRevision: 0},
		}},
		Success: []*etcdserverpb.RequestOp{{
			Request: &etcdserverpb.RequestOp_RequestPut{
				RequestPut: &etcdserverpb.PutRequest{
					Key: []byte("/allowed/k"), Value: []byte("v"),
				},
			},
		}},
	})
	require.NoError(t, err)
	assert.True(t, resp.GetSucceeded())
	assert.Equal(t, int64(4), resp.GetHeader().GetRevision())
	assert.Equal(t, 1, uc.txnCallCount())
}

// -----------------------------------------------------------------------------
// Txn — DTO→proto conversion
// -----------------------------------------------------------------------------

func TestKVServer_Txn_ConvertsResult(t *testing.T) {
	t.Parallel()

	kv := &domain.KVPair{
		Namespace:      "ns",
		Path:           "/a",
		Value:          []byte("v"),
		CreateRevision: 1,
		ModRevision:    2,
		Version:        2,
	}

	tests := []struct {
		name   string
		req    *etcdserverpb.TxnRequest
		result configuc.KVTxnResult
		assert func(*testing.T, *etcdserverpb.TxnResponse)
	}{
		{
			// buildRangeResponse is shared with the standalone Range RPC, so
			// count-only shaping must apply to a nested range identically.
			name: "count-only range nested in a txn sets Count and drops Kvs",
			req: &etcdserverpb.TxnRequest{
				Success: []*etcdserverpb.RequestOp{{
					Request: &etcdserverpb.RequestOp_RequestRange{
						RequestRange: &etcdserverpb.RangeRequest{
							Key: []byte("/ns/a"), CountOnly: true,
						},
					},
				}},
			},
			result: configuc.KVTxnResult{
				Succeeded: true,
				Revision:  2,
				Responses: []configuc.KVOpResult{{
					Kind:  configuc.KVOpRange,
					Range: &configuc.KVRangeResult{KVs: []*domain.KVPair{kv}, Revision: 2},
				}},
			},
			assert: func(t *testing.T, resp *etcdserverpb.TxnResponse) {
				t.Helper()

				rr, ok := resp.GetResponses()[0].
					GetResponse().(*etcdserverpb.ResponseOp_ResponseRange)
				require.True(t, ok)
				assert.Equal(t, int64(1), rr.ResponseRange.GetCount())
				assert.Nil(t, rr.ResponseRange.GetKvs(), "CountOnly must drop the pairs")
			},
		},
		{
			name: "range nested in a txn is sorted by the request's sort order",
			req: &etcdserverpb.TxnRequest{
				Success: []*etcdserverpb.RequestOp{{
					Request: &etcdserverpb.RequestOp_RequestRange{
						RequestRange: &etcdserverpb.RangeRequest{
							Key:        []byte("/ns/"),
							RangeEnd:   []byte("/ns0"),
							SortOrder:  etcdserverpb.RangeRequest_DESCEND,
							SortTarget: etcdserverpb.RangeRequest_KEY,
						},
					},
				}},
			},
			result: configuc.KVTxnResult{
				Succeeded: true,
				Revision:  3,
				Responses: []configuc.KVOpResult{{
					Kind: configuc.KVOpRange,
					Range: &configuc.KVRangeResult{
						KVs: []*domain.KVPair{
							{Namespace: "ns", Path: "/a"},
							{Namespace: "ns", Path: "/b"},
						},
						Revision: 3,
						More:     true,
					},
				}},
			},
			assert: func(t *testing.T, resp *etcdserverpb.TxnResponse) {
				t.Helper()

				rr, ok := resp.GetResponses()[0].
					GetResponse().(*etcdserverpb.ResponseOp_ResponseRange)
				require.True(t, ok)
				require.Len(t, rr.ResponseRange.GetKvs(), 2)
				assert.Equal(t, []byte("/ns/b"), rr.ResponseRange.GetKvs()[0].GetKey())
				assert.True(t, rr.ResponseRange.GetMore())
			},
		},
		{
			name: "put response carries prev_kv only when asked",
			req: &etcdserverpb.TxnRequest{
				Success: []*etcdserverpb.RequestOp{
					{Request: &etcdserverpb.RequestOp_RequestPut{
						RequestPut: &etcdserverpb.PutRequest{
							Key: []byte("/ns/a"), Value: []byte("v"), PrevKv: true,
						},
					}},
					{Request: &etcdserverpb.RequestOp_RequestPut{
						RequestPut: &etcdserverpb.PutRequest{
							Key: []byte("/ns/b"), Value: []byte("v"),
						},
					}},
				},
			},
			result: configuc.KVTxnResult{
				Succeeded: true,
				Revision:  5,
				Responses: []configuc.KVOpResult{
					{Kind: configuc.KVOpPut, Put: &configuc.KVPutResult{Prev: kv, Revision: 5}},
					{Kind: configuc.KVOpPut, Put: &configuc.KVPutResult{Prev: kv, Revision: 5}},
				},
			},
			assert: func(t *testing.T, resp *etcdserverpb.TxnResponse) {
				t.Helper()

				withPrev, ok := resp.GetResponses()[0].
					GetResponse().(*etcdserverpb.ResponseOp_ResponsePut)
				require.True(t, ok)
				require.NotNil(t, withPrev.ResponsePut.GetPrevKv())
				assert.Equal(t, []byte("/ns/a"), withPrev.ResponsePut.GetPrevKv().GetKey())

				withoutPrev, ok := resp.GetResponses()[1].
					GetResponse().(*etcdserverpb.ResponseOp_ResponsePut)
				require.True(t, ok)
				assert.Nil(t, withoutPrev.ResponsePut.GetPrevKv())
			},
		},
		{
			// Succeeded == false means the results belong to the Failure
			// requests — zipping them against Success would mis-shape them.
			name: "failure branch results are zipped with the failure requests",
			req: &etcdserverpb.TxnRequest{
				Success: []*etcdserverpb.RequestOp{{
					Request: &etcdserverpb.RequestOp_RequestPut{
						RequestPut: &etcdserverpb.PutRequest{Key: []byte("/ns/a")},
					},
				}},
				Failure: []*etcdserverpb.RequestOp{{
					Request: &etcdserverpb.RequestOp_RequestDeleteRange{
						RequestDeleteRange: &etcdserverpb.DeleteRangeRequest{
							Key: []byte("/ns/a"), PrevKv: true,
						},
					},
				}},
			},
			result: configuc.KVTxnResult{
				Succeeded: false,
				Revision:  6,
				Responses: []configuc.KVOpResult{{
					Kind: configuc.KVOpDeleteRange,
					DeleteRange: &configuc.KVDeleteRangeResult{
						Deleted:  []*domain.KVPair{kv},
						Revision: 6,
					},
				}},
			},
			assert: func(t *testing.T, resp *etcdserverpb.TxnResponse) {
				t.Helper()

				assert.False(t, resp.GetSucceeded())

				dr, ok := resp.GetResponses()[0].
					GetResponse().(*etcdserverpb.ResponseOp_ResponseDeleteRange)
				require.True(t, ok)
				assert.Equal(t, int64(1), dr.ResponseDeleteRange.GetDeleted())
				require.Len(t, dr.ResponseDeleteRange.GetPrevKvs(), 1)
			},
		},
		{
			name: "nested txn result converts recursively",
			req: &etcdserverpb.TxnRequest{
				Success: []*etcdserverpb.RequestOp{{
					Request: &etcdserverpb.RequestOp_RequestTxn{
						RequestTxn: &etcdserverpb.TxnRequest{
							Success: []*etcdserverpb.RequestOp{{
								Request: &etcdserverpb.RequestOp_RequestPut{
									RequestPut: &etcdserverpb.PutRequest{Key: []byte("/ns/a")},
								},
							}},
						},
					},
				}},
			},
			result: configuc.KVTxnResult{
				Succeeded: true,
				Revision:  7,
				Responses: []configuc.KVOpResult{{
					Kind: configuc.KVOpTxn,
					Txn: &configuc.KVTxnResult{
						Succeeded: true,
						Revision:  7,
						Responses: []configuc.KVOpResult{{
							Kind: configuc.KVOpPut,
							Put:  &configuc.KVPutResult{Revision: 7},
						}},
					},
				}},
			},
			assert: func(t *testing.T, resp *etcdserverpb.TxnResponse) {
				t.Helper()

				inner, ok := resp.GetResponses()[0].
					GetResponse().(*etcdserverpb.ResponseOp_ResponseTxn)
				require.True(t, ok)
				assert.True(t, inner.ResponseTxn.GetSucceeded())
				require.Len(t, inner.ResponseTxn.GetResponses(), 1)
			},
		},
		{
			name:   "empty transaction returns the usecase's revision and no responses",
			req:    &etcdserverpb.TxnRequest{},
			result: configuc.KVTxnResult{Succeeded: true, Revision: 5},
			assert: func(t *testing.T, resp *etcdserverpb.TxnResponse) {
				t.Helper()

				assert.True(t, resp.GetSucceeded())
				assert.Equal(t, int64(5), resp.GetHeader().GetRevision())
				assert.Empty(t, resp.GetResponses())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := newFakeKVUsecase()
			uc.txnResult = tt.result
			s := etcdv3.NewKVServer(uc)

			resp, err := s.Txn(t.Context(), tt.req)
			require.NoError(t, err)

			tt.assert(t, resp)
		})
	}
}

func TestKVServer_Txn_UsecaseError(t *testing.T) {
	t.Parallel()

	uc := newFakeKVUsecase()
	uc.txnErr = errors.New("boom")
	s := etcdv3.NewKVServer(uc)

	_, err := s.Txn(t.Context(), &etcdserverpb.TxnRequest{
		Success: []*etcdserverpb.RequestOp{{
			Request: &etcdserverpb.RequestOp_RequestPut{
				RequestPut: &etcdserverpb.PutRequest{Key: []byte("/ns/a"), Value: []byte("v")},
			},
		}},
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "boom")
}

func TestKVServer_Txn_LockedKeyIsFailedPrecondition(t *testing.T) {
	t.Parallel()

	uc := newFakeKVUsecase()
	uc.txnErr = domain.ErrLocked
	s := etcdv3.NewKVServer(uc)

	_, err := s.Txn(t.Context(), &etcdserverpb.TxnRequest{
		Success: []*etcdserverpb.RequestOp{{
			Request: &etcdserverpb.RequestOp_RequestPut{
				RequestPut: &etcdserverpb.PutRequest{Key: []byte("/ns/a"), Value: []byte("v")},
			},
		}},
	})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestKVServer_Txn_ResponseArityMismatch(t *testing.T) {
	t.Parallel()

	// The usecase must return one result per op in the branch it ran. A
	// mismatch would silently mis-shape the wire response, so it is reported
	// rather than zipped over.
	uc := newFakeKVUsecase()
	uc.txnResult = configuc.KVTxnResult{Succeeded: true, Revision: 1}
	s := etcdv3.NewKVServer(uc)

	_, err := s.Txn(t.Context(), &etcdserverpb.TxnRequest{
		Success: []*etcdserverpb.RequestOp{{
			Request: &etcdserverpb.RequestOp_RequestPut{
				RequestPut: &etcdserverpb.PutRequest{Key: []byte("/ns/a"), Value: []byte("v")},
			},
		}},
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "txn response mismatch")
}
