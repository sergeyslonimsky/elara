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

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/etcdv3"
)

// fakeKVRepo is an in-memory KVRepo suitable for testing KVServer logic
// without touching bbolt. It stores pairs keyed by (namespace, path).
type fakeKVRepo struct {
	mu    sync.Mutex
	pairs map[string]*domain.KVPair // key: ns+"\x00"+path
	rev   int64

	// injection hooks for error paths
	rangeErr    error
	putErr      error
	deleteErr   error
	currentErr  error
	interceptOp func() // called at start of each mutation
}

func newFakeKVRepo() *fakeKVRepo {
	return &fakeKVRepo{pairs: make(map[string]*domain.KVPair)}
}

func (f *fakeKVRepo) CurrentRevisionValue(_ context.Context) (int64, error) {
	if f.currentErr != nil {
		return 0, f.currentErr
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	return f.rev, nil
}

func (f *fakeKVRepo) RangeQuery(
	_ context.Context,
	startNS, startPath string,
	endNS, endPath string,
	limit int64,
	_ int64,
	keysOnly bool,
) ([]*domain.KVPair, bool, error) {
	if f.rangeErr != nil {
		return nil, false, f.rangeErr
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	var results []*domain.KVPair

	for _, k := range f.matchKeys(startNS, startPath, endNS, endPath) {
		clone := *f.pairs[k]
		if keysOnly {
			clone.Value = nil
		}

		results = append(results, &clone)
	}

	sortByKey(results)

	more := false
	if limit > 0 && int64(len(results)) > limit {
		results = results[:limit]
		more = true
	}

	return results, more, nil
}

func (f *fakeKVRepo) PutKey(
	_ context.Context,
	namespace, path string,
	value []byte,
) (*domain.KVPair, int64, error) {
	if f.interceptOp != nil {
		f.interceptOp()
	}

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
	f.pairs[k] = &domain.KVPair{
		Namespace:      namespace,
		Path:           path,
		Value:          valCopy,
		CreateRevision: prev.CreateRevision,
		ModRevision:    f.rev,
		Version:        prev.Version + 1,
	}

	return new(*prev), f.rev, nil
}

func (f *fakeKVRepo) DeleteRangeKeys(
	_ context.Context,
	startNS, startPath string,
	endNS, endPath string,
	returnPrev bool,
) ([]*domain.KVPair, int64, error) {
	if f.interceptOp != nil {
		f.interceptOp()
	}

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
			deleted = append(deleted, new(*kv))
		} else {
			deleted = append(deleted, &domain.KVPair{Namespace: kv.Namespace, Path: kv.Path})
		}
	}

	sortByKey(deleted)

	return deleted, f.rev, nil
}

func (f *fakeKVRepo) key(ns, path string) string { return ns + "\x00" + path }

// matchKeys returns the sorted list of map keys that fall within the given
// etcd-style [start, end) range. Centralises the range-match logic so
// RangeQuery and DeleteRangeKeys don't each re-implement it.
func (f *fakeKVRepo) matchKeys(startNS, startPath, endNS, endPath string) []string {
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

// fakePublisher records notifications for assertions.
type fakePublisher struct {
	mu      sync.Mutex
	created []*domain.Config
	updated []*domain.Config
	deleted []deletedNotify
}

type deletedNotify struct {
	path, namespace string
	revision        int64
}

func (f *fakePublisher) NotifyCreated(_ context.Context, cfg *domain.Config) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, cfg)
}

func (f *fakePublisher) NotifyUpdated(_ context.Context, cfg *domain.Config) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updated = append(f.updated, cfg)
}

func (f *fakePublisher) NotifyDeleted(_ context.Context, path, ns string, rev int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, deletedNotify{path, ns, rev})
}

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

func TestKVServer_Put_CreatesNewKey(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	pub := &fakePublisher{}
	s := etcdv3.NewKVServer(repo, pub)

	resp, err := s.Put(context.Background(), &etcdserverpb.PutRequest{
		Key:   []byte("/default/foo.json"),
		Value: []byte("hello"),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Header.Revision)
	assert.Nil(t, resp.PrevKv)

	assert.Len(t, pub.created, 1, "publisher.NotifyCreated called")
	assert.Empty(t, pub.updated, "publisher.NotifyUpdated NOT called on create")
	assert.Equal(t, "default", pub.created[0].Namespace)
	assert.Equal(t, "/foo.json", pub.created[0].Path)
	assert.Equal(t, int64(1), pub.created[0].Version)
	assert.Equal(t, int64(1), pub.created[0].CreateRevision)
	assert.Equal(t, int64(1), pub.created[0].Revision)
}

func TestKVServer_Put_UpdatesExistingKey_WithPrevKv(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	pub := &fakePublisher{}
	s := etcdv3.NewKVServer(repo, pub)

	_, err := s.Put(context.Background(), &etcdserverpb.PutRequest{
		Key: []byte("/default/foo"), Value: []byte("v1"),
	})
	require.NoError(t, err)

	resp, err := s.Put(context.Background(), &etcdserverpb.PutRequest{
		Key: []byte("/default/foo"), Value: []byte("v2"), PrevKv: true,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.Header.Revision)
	require.NotNil(t, resp.PrevKv)
	assert.Equal(t, []byte("v1"), resp.PrevKv.Value)
	assert.Equal(t, int64(1), resp.PrevKv.Version)

	assert.Len(t, pub.created, 1)
	assert.Len(t, pub.updated, 1, "second Put notifies Updated, not Created")
	assert.Equal(t, int64(2), pub.updated[0].Version)
	assert.Equal(t, int64(1), pub.updated[0].CreateRevision, "CreateRevision preserved on update")
}

func TestKVServer_Put_InvalidKey(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVRepo(), nil)

	_, err := s.Put(context.Background(), &etcdserverpb.PutRequest{Key: []byte("bad")})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestKVServer_Put_IgnoreValueUnsupported(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVRepo(), nil)

	_, err := s.Put(context.Background(), &etcdserverpb.PutRequest{
		Key: []byte("/ns/x"), IgnoreValue: true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Unimplemented, status.Code(err))
}

func TestKVServer_Put_RepoError(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	repo.putErr = errors.New("boom")
	s := etcdv3.NewKVServer(repo, &fakePublisher{})

	_, err := s.Put(context.Background(), &etcdserverpb.PutRequest{
		Key: []byte("/ns/x"), Value: []byte("v"),
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestKVServer_Range_SingleKey(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	s := etcdv3.NewKVServer(repo, &fakePublisher{})

	_, err := s.Put(context.Background(), &etcdserverpb.PutRequest{
		Key: []byte("/default/foo"), Value: []byte("v1"),
	})
	require.NoError(t, err)

	resp, err := s.Range(context.Background(), &etcdserverpb.RangeRequest{
		Key: []byte("/default/foo"),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Count)
	require.Len(t, resp.Kvs, 1)
	assert.Equal(t, []byte("/default/foo"), resp.Kvs[0].Key)
	assert.Equal(t, []byte("v1"), resp.Kvs[0].Value)
}

func TestKVServer_Range_Prefix(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	s := etcdv3.NewKVServer(repo, &fakePublisher{})

	ctx := context.Background()
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
	assert.Equal(t, int64(3), resp.Count)
	assert.Len(t, resp.Kvs, 3)
}

func TestKVServer_Range_Limit_ReportsMore(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	s := etcdv3.NewKVServer(repo, &fakePublisher{})

	ctx := context.Background()
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
	assert.Len(t, resp.Kvs, 2)
	assert.True(t, resp.More, "More should be true when results truncated")
}

func TestKVServer_Range_CountOnly(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	s := etcdv3.NewKVServer(repo, &fakePublisher{})
	ctx := context.Background()

	for _, k := range []string{"/ns/a", "/ns/b"} {
		_, err := s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte(k), Value: []byte("v")})
		require.NoError(t, err)
	}

	resp, err := s.Range(ctx, &etcdserverpb.RangeRequest{
		Key: []byte("/ns/"), RangeEnd: []byte("/ns0"), CountOnly: true,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.Count)
	assert.Empty(t, resp.Kvs)
}

func TestKVServer_Range_KeysOnly(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	s := etcdv3.NewKVServer(repo, &fakePublisher{})
	ctx := context.Background()

	_, err := s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte("/ns/a"), Value: []byte("v1")})
	require.NoError(t, err)

	resp, err := s.Range(ctx, &etcdserverpb.RangeRequest{
		Key: []byte("/ns/a"), KeysOnly: true,
	})
	require.NoError(t, err)
	require.Len(t, resp.Kvs, 1)
	assert.Empty(t, resp.Kvs[0].Value, "KeysOnly must strip Value")
}

func TestKVServer_Range_Sort_Descend(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	s := etcdv3.NewKVServer(repo, &fakePublisher{})
	ctx := context.Background()

	for _, k := range []string{"/ns/a", "/ns/b", "/ns/c"} {
		_, err := s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte(k), Value: []byte("v")})
		require.NoError(t, err)
	}

	resp, err := s.Range(ctx, &etcdserverpb.RangeRequest{
		Key: []byte("/ns/"), RangeEnd: []byte("/ns0"),
		SortOrder: etcdserverpb.RangeRequest_DESCEND, SortTarget: etcdserverpb.RangeRequest_KEY,
	})
	require.NoError(t, err)
	require.Len(t, resp.Kvs, 3)
	assert.Equal(t, []byte("/ns/c"), resp.Kvs[0].Key)
	assert.Equal(t, []byte("/ns/a"), resp.Kvs[2].Key)
}

func TestKVServer_Range_InvalidKey(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVRepo(), &fakePublisher{})

	_, err := s.Range(context.Background(), &etcdserverpb.RangeRequest{Key: []byte("bad")})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestKVServer_Range_RepoError(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	repo.rangeErr = errors.New("boom")
	s := etcdv3.NewKVServer(repo, &fakePublisher{})

	_, err := s.Range(context.Background(), &etcdserverpb.RangeRequest{Key: []byte("/ns/x")})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "boom")
}

func TestKVServer_DeleteRange_SingleKey(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	pub := &fakePublisher{}
	s := etcdv3.NewKVServer(repo, pub)
	ctx := context.Background()

	_, err := s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte("/ns/x"), Value: []byte("v")})
	require.NoError(t, err)

	resp, err := s.DeleteRange(ctx, &etcdserverpb.DeleteRangeRequest{
		Key: []byte("/ns/x"), PrevKv: true,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Deleted)
	require.Len(t, resp.PrevKvs, 1)
	assert.Equal(t, []byte("v"), resp.PrevKvs[0].Value)

	assert.Len(t, pub.deleted, 1)
	assert.Equal(t, "/x", pub.deleted[0].path)
	assert.Equal(t, "ns", pub.deleted[0].namespace)
	assert.Equal(t, int64(2), pub.deleted[0].revision, "delete must carry the new revision")
}

func TestKVServer_DeleteRange_Nothing_ReturnsCurrentRev(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	repo.rev = 7
	pub := &fakePublisher{}
	s := etcdv3.NewKVServer(repo, pub)

	resp, err := s.DeleteRange(context.Background(), &etcdserverpb.DeleteRangeRequest{
		Key: []byte("/ns/missing"),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), resp.Deleted)
	assert.Equal(t, int64(7), resp.Header.Revision)
	assert.Empty(t, pub.deleted, "no publisher notifications when nothing deleted")
}

func TestKVServer_DeleteRange_Prefix(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	pub := &fakePublisher{}
	s := etcdv3.NewKVServer(repo, pub)
	ctx := context.Background()

	for _, k := range []string{"/ns/a", "/ns/b", "/other/c"} {
		_, err := s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte(k), Value: []byte("v")})
		require.NoError(t, err)
	}

	resp, err := s.DeleteRange(ctx, &etcdserverpb.DeleteRangeRequest{
		Key: []byte("/ns/"), RangeEnd: []byte("/ns0"),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.Deleted)
	assert.Len(t, pub.deleted, 2)
}

func TestKVServer_DeleteRange_InvalidKey(t *testing.T) {
	t.Parallel()

	s := etcdv3.NewKVServer(newFakeKVRepo(), &fakePublisher{})

	_, err := s.DeleteRange(context.Background(), &etcdserverpb.DeleteRangeRequest{Key: []byte("bad")})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestKVServer_Compact_IsNoOp(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	repo.rev = 42
	s := etcdv3.NewKVServer(repo, &fakePublisher{})

	resp, err := s.Compact(context.Background(), &etcdserverpb.CompactionRequest{Revision: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(42), resp.Header.Revision)
}

// -----------------------------------------------------------------------------
// Txn
// -----------------------------------------------------------------------------

func TestKVServer_Txn_SuccessBranch(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	s := etcdv3.NewKVServer(repo, &fakePublisher{})
	ctx := context.Background()

	_, err := s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte("/ns/k"), Value: []byte("hello")})
	require.NoError(t, err)

	// If value == "hello" then put /ns/k2
	resp, err := s.Txn(ctx, &etcdserverpb.TxnRequest{
		Compare: []*etcdserverpb.Compare{{
			Key:         []byte("/ns/k"),
			Target:      etcdserverpb.Compare_VALUE,
			Result:      etcdserverpb.Compare_EQUAL,
			TargetUnion: &etcdserverpb.Compare_Value{Value: []byte("hello")},
		}},
		Success: []*etcdserverpb.RequestOp{{
			Request: &etcdserverpb.RequestOp_RequestPut{RequestPut: &etcdserverpb.PutRequest{
				Key: []byte("/ns/k2"), Value: []byte("ok"),
			}},
		}},
		Failure: []*etcdserverpb.RequestOp{{
			Request: &etcdserverpb.RequestOp_RequestRange{RequestRange: &etcdserverpb.RangeRequest{
				Key: []byte("/ns/k"),
			}},
		}},
	})
	require.NoError(t, err)
	assert.True(t, resp.Succeeded)
	require.Len(t, resp.Responses, 1)
	_, isPut := resp.Responses[0].Response.(*etcdserverpb.ResponseOp_ResponsePut)
	assert.True(t, isPut)

	rg, err := s.Range(ctx, &etcdserverpb.RangeRequest{Key: []byte("/ns/k2")})
	require.NoError(t, err)
	assert.Equal(t, int64(1), rg.Count)
}

func TestKVServer_Txn_FailureBranch(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	s := etcdv3.NewKVServer(repo, &fakePublisher{})
	ctx := context.Background()

	_, err := s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte("/ns/k"), Value: []byte("hello")})
	require.NoError(t, err)

	// Compare mismatches → failure branch executes (just a Range here).
	resp, err := s.Txn(ctx, &etcdserverpb.TxnRequest{
		Compare: []*etcdserverpb.Compare{{
			Key: []byte("/ns/k"), Target: etcdserverpb.Compare_VALUE, Result: etcdserverpb.Compare_EQUAL,
			TargetUnion: &etcdserverpb.Compare_Value{Value: []byte("different")},
		}},
		Success: []*etcdserverpb.RequestOp{{
			Request: &etcdserverpb.RequestOp_RequestPut{RequestPut: &etcdserverpb.PutRequest{
				Key: []byte("/ns/k3"), Value: []byte("should-not-exist"),
			}},
		}},
		Failure: []*etcdserverpb.RequestOp{{
			Request: &etcdserverpb.RequestOp_RequestRange{RequestRange: &etcdserverpb.RangeRequest{
				Key: []byte("/ns/k"),
			}},
		}},
	})
	require.NoError(t, err)
	assert.False(t, resp.Succeeded)

	// Ensure the Success branch didn't leak a side effect.
	rg, err := s.Range(ctx, &etcdserverpb.RangeRequest{Key: []byte("/ns/k3")})
	require.NoError(t, err)
	assert.Equal(t, int64(0), rg.Count)
}

func TestKVServer_Txn_ConjunctionShortCircuit(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	s := etcdv3.NewKVServer(repo, &fakePublisher{})
	ctx := context.Background()

	_, err := s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte("/ns/k"), Value: []byte("v")})
	require.NoError(t, err)

	// Two compares: first true, second false → overall false.
	resp, err := s.Txn(ctx, &etcdserverpb.TxnRequest{
		Compare: []*etcdserverpb.Compare{
			{
				Key: []byte("/ns/k"), Target: etcdserverpb.Compare_VALUE, Result: etcdserverpb.Compare_EQUAL,
				TargetUnion: &etcdserverpb.Compare_Value{Value: []byte("v")},
			},
			{
				Key: []byte("/ns/k"), Target: etcdserverpb.Compare_VERSION, Result: etcdserverpb.Compare_EQUAL,
				TargetUnion: &etcdserverpb.Compare_Version{Version: 999},
			},
		},
	})
	require.NoError(t, err)
	assert.False(t, resp.Succeeded)
}

func TestKVServer_Txn_MissingKey_AgainstCreateRev0(t *testing.T) {
	t.Parallel()

	// Canonical etcd idiom: assert "key does not exist" via Compare(CreateRevision, =, 0)
	repo := newFakeKVRepo()
	s := etcdv3.NewKVServer(repo, &fakePublisher{})

	resp, err := s.Txn(context.Background(), &etcdserverpb.TxnRequest{
		Compare: []*etcdserverpb.Compare{{
			Key: []byte("/ns/new"), Target: etcdserverpb.Compare_CREATE, Result: etcdserverpb.Compare_EQUAL,
			TargetUnion: &etcdserverpb.Compare_CreateRevision{CreateRevision: 0},
		}},
		Success: []*etcdserverpb.RequestOp{{
			Request: &etcdserverpb.RequestOp_RequestPut{RequestPut: &etcdserverpb.PutRequest{
				Key: []byte("/ns/new"), Value: []byte("v"),
			}},
		}},
	})
	require.NoError(t, err)
	assert.True(t, resp.Succeeded)
}

func TestKVServer_Txn_EmptyOps_ReturnsCurrentRev(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	repo.rev = 5
	s := etcdv3.NewKVServer(repo, &fakePublisher{})

	resp, err := s.Txn(context.Background(), &etcdserverpb.TxnRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Succeeded, "no compares → vacuously true")
	assert.Equal(t, int64(5), resp.Header.Revision)
	assert.Empty(t, resp.Responses)
}

func TestKVServer_Txn_NestedTxn(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	s := etcdv3.NewKVServer(repo, &fakePublisher{})

	inner := &etcdserverpb.TxnRequest{
		Success: []*etcdserverpb.RequestOp{{
			Request: &etcdserverpb.RequestOp_RequestPut{RequestPut: &etcdserverpb.PutRequest{
				Key: []byte("/ns/nested"), Value: []byte("v"),
			}},
		}},
	}

	resp, err := s.Txn(context.Background(), &etcdserverpb.TxnRequest{
		Success: []*etcdserverpb.RequestOp{{
			Request: &etcdserverpb.RequestOp_RequestTxn{RequestTxn: inner},
		}},
	})
	require.NoError(t, err)
	assert.True(t, resp.Succeeded)
	require.Len(t, resp.Responses, 1)
	_, ok := resp.Responses[0].Response.(*etcdserverpb.ResponseOp_ResponseTxn)
	assert.True(t, ok)
}

func TestKVServer_Txn_CompareInvalidKey(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	s := etcdv3.NewKVServer(repo, &fakePublisher{})

	_, err := s.Txn(context.Background(), &etcdserverpb.TxnRequest{
		Compare: []*etcdserverpb.Compare{{Key: []byte("bad")}},
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestKVServer_Txn_CompareWithRange_AllMatch(t *testing.T) {
	t.Parallel()

	// Compare against a range — all keys in range must satisfy the predicate.
	repo := newFakeKVRepo()
	s := etcdv3.NewKVServer(repo, &fakePublisher{})
	ctx := context.Background()

	for _, k := range []string{"/ns/a", "/ns/b"} {
		_, err := s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte(k), Value: []byte("same")})
		require.NoError(t, err)
	}

	resp, err := s.Txn(ctx, &etcdserverpb.TxnRequest{
		Compare: []*etcdserverpb.Compare{{
			Key: []byte("/ns/"), RangeEnd: []byte("/ns0"),
			Target: etcdserverpb.Compare_VALUE, Result: etcdserverpb.Compare_EQUAL,
			TargetUnion: &etcdserverpb.Compare_Value{Value: []byte("same")},
		}},
	})
	require.NoError(t, err)
	assert.True(t, resp.Succeeded)

	// Now add a mismatching value and re-run.
	_, err = s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte("/ns/c"), Value: []byte("different")})
	require.NoError(t, err)

	resp2, err := s.Txn(ctx, &etcdserverpb.TxnRequest{
		Compare: []*etcdserverpb.Compare{{
			Key: []byte("/ns/"), RangeEnd: []byte("/ns0"),
			Target: etcdserverpb.Compare_VALUE, Result: etcdserverpb.Compare_EQUAL,
			TargetUnion: &etcdserverpb.Compare_Value{Value: []byte("same")},
		}},
	})
	require.NoError(t, err)
	assert.False(t, resp2.Succeeded, "one mismatching value in range must fail the compare")
}

func TestKVServer_Txn_RunOp_Range(t *testing.T) {
	t.Parallel()

	resp := txnWithSingleOp(t, "/ns/a", &etcdserverpb.RequestOp{
		Request: &etcdserverpb.RequestOp_RequestRange{
			RequestRange: &etcdserverpb.RangeRequest{Key: []byte("/ns/a")},
		},
	})

	rr, ok := resp.Responses[0].Response.(*etcdserverpb.ResponseOp_ResponseRange)
	require.True(t, ok)
	assert.Equal(t, int64(1), rr.ResponseRange.Count)
}

func TestKVServer_Txn_RunOp_DeleteRange(t *testing.T) {
	t.Parallel()

	resp := txnWithSingleOp(t, "/ns/x", &etcdserverpb.RequestOp{
		Request: &etcdserverpb.RequestOp_RequestDeleteRange{
			RequestDeleteRange: &etcdserverpb.DeleteRangeRequest{Key: []byte("/ns/x")},
		},
	})

	dr, ok := resp.Responses[0].Response.(*etcdserverpb.ResponseOp_ResponseDeleteRange)
	require.True(t, ok)
	assert.Equal(t, int64(1), dr.ResponseDeleteRange.Deleted)
}

// txnWithSingleOp seeds a key, runs a Txn with a single success op, and
// returns the response. Shared setup for RunOp_Range and RunOp_DeleteRange.
func txnWithSingleOp(t *testing.T, key string, op *etcdserverpb.RequestOp) *etcdserverpb.TxnResponse {
	t.Helper()

	repo := newFakeKVRepo()
	s := etcdv3.NewKVServer(repo, &fakePublisher{})
	ctx := context.Background()

	_, err := s.Put(ctx, &etcdserverpb.PutRequest{Key: []byte(key), Value: []byte("v")})
	require.NoError(t, err)

	resp, err := s.Txn(ctx, &etcdserverpb.TxnRequest{
		Success: []*etcdserverpb.RequestOp{op},
	})
	require.NoError(t, err)
	require.Len(t, resp.Responses, 1)

	return resp
}

func TestKVServer_Txn_RunOp_PropagatesError(t *testing.T) {
	t.Parallel()

	// If any op in the chosen branch fails, Txn must return an error.
	repo := newFakeKVRepo()
	s := etcdv3.NewKVServer(repo, &fakePublisher{})

	// Put with invalid key must cause runOp → Put → error.
	_, err := s.Txn(context.Background(), &etcdserverpb.TxnRequest{
		Success: []*etcdserverpb.RequestOp{{
			Request: &etcdserverpb.RequestOp_RequestPut{RequestPut: &etcdserverpb.PutRequest{
				Key: []byte("bad"),
			}},
		}},
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestKVServer_Txn_RunOp_UnknownRequestType(t *testing.T) {
	t.Parallel()

	// An empty request op (nil union) must be rejected.
	s := etcdv3.NewKVServer(newFakeKVRepo(), &fakePublisher{})

	_, err := s.Txn(context.Background(), &etcdserverpb.TxnRequest{
		Success: []*etcdserverpb.RequestOp{{Request: nil}},
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestKVServer_DeleteRange_CurrentRevError(t *testing.T) {
	t.Parallel()

	// When no keys match AND CurrentRevisionValue errors, DeleteRange returns Internal.
	repo := newFakeKVRepo()
	repo.currentErr = errors.New("boom")
	s := etcdv3.NewKVServer(repo, &fakePublisher{})

	_, err := s.DeleteRange(context.Background(), &etcdserverpb.DeleteRangeRequest{Key: []byte("/ns/x")})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestKVServer_Range_CurrentRevError(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	repo.currentErr = errors.New("boom")
	s := etcdv3.NewKVServer(repo, &fakePublisher{})

	_, err := s.Range(context.Background(), &etcdserverpb.RangeRequest{Key: []byte("/ns/x")})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestKVServer_Compact_CurrentRevError(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	repo.currentErr = errors.New("boom")
	s := etcdv3.NewKVServer(repo, &fakePublisher{})

	_, err := s.Compact(context.Background(), &etcdserverpb.CompactionRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestKVServer_DeleteRange_RepoError(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	repo.deleteErr = errors.New("boom")
	s := etcdv3.NewKVServer(repo, &fakePublisher{})

	_, err := s.DeleteRange(context.Background(), &etcdserverpb.DeleteRangeRequest{Key: []byte("/ns/x")})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestKVServer_EvalCompare_PropagatesRangeError(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	repo.rangeErr = errors.New("db failed")
	s := etcdv3.NewKVServer(repo, &fakePublisher{})

	_, err := s.Txn(context.Background(), &etcdserverpb.TxnRequest{
		Compare: []*etcdserverpb.Compare{{
			Key: []byte("/ns/x"), Target: etcdserverpb.Compare_VERSION,
			TargetUnion: &etcdserverpb.Compare_Version{Version: 0},
		}},
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "db failed")
}

func TestKVServer_Put_NilPublisher(t *testing.T) {
	t.Parallel()

	// Regression guard: notifyPut must tolerate a nil publisher.
	repo := newFakeKVRepo()
	s := etcdv3.NewKVServer(repo, nil)

	_, err := s.Put(context.Background(), &etcdserverpb.PutRequest{
		Key: []byte("/ns/x"), Value: []byte("v"),
	})
	require.NoError(t, err)
}

func TestKVServer_DeleteRange_NilPublisher(t *testing.T) {
	t.Parallel()

	repo := newFakeKVRepo()
	s := etcdv3.NewKVServer(repo, nil)

	_, err := s.Put(context.Background(), &etcdserverpb.PutRequest{
		Key: []byte("/ns/x"), Value: []byte("v"),
	})
	require.NoError(t, err)

	_, err = s.DeleteRange(context.Background(), &etcdserverpb.DeleteRangeRequest{
		Key: []byte("/ns/x"),
	})
	require.NoError(t, err)
}
