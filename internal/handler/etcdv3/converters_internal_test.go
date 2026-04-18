package etcdv3

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestKvPairToProto(t *testing.T) {
	t.Parallel()

	kv := &domain.KVPair{
		Namespace:      "default",
		Path:           "/foo.json",
		Value:          []byte(`{"x":1}`),
		CreateRevision: 10,
		ModRevision:    15,
		Version:        3,
	}

	got := kvPairToProto(kv)

	assert.Equal(t, []byte("/default/foo.json"), got.Key)
	assert.Equal(t, []byte(`{"x":1}`), got.Value)
	assert.Equal(t, int64(10), got.CreateRevision)
	assert.Equal(t, int64(15), got.ModRevision)
	assert.Equal(t, int64(3), got.Version)
}

func TestNewHeader(t *testing.T) {
	t.Parallel()

	h := newHeader(42)

	assert.Equal(t, int64(42), h.Revision)
	assert.Equal(t, clusterID, h.ClusterId)
	assert.Equal(t, memberID, h.MemberId)
	assert.Equal(t, raftTerm, h.RaftTerm)
}

func TestCompareInt64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		op        etcdserverpb.Compare_CompareResult
		got, want int64
		expected  bool
	}{
		{etcdserverpb.Compare_EQUAL, 5, 5, true},
		{etcdserverpb.Compare_EQUAL, 5, 6, false},
		{etcdserverpb.Compare_NOT_EQUAL, 5, 6, true},
		{etcdserverpb.Compare_NOT_EQUAL, 5, 5, false},
		{etcdserverpb.Compare_GREATER, 6, 5, true},
		{etcdserverpb.Compare_GREATER, 5, 5, false},
		{etcdserverpb.Compare_GREATER, 4, 5, false},
		{etcdserverpb.Compare_LESS, 4, 5, true},
		{etcdserverpb.Compare_LESS, 5, 5, false},
	}

	for _, tc := range tests {
		got := compareInt64(tc.op, tc.got, tc.want)
		assert.Equal(t, tc.expected, got, "op=%v got=%d want=%d", tc.op, tc.got, tc.want)
	}
}

func TestCompareBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		op        etcdserverpb.Compare_CompareResult
		got, want []byte
		expected  bool
	}{
		{etcdserverpb.Compare_EQUAL, []byte("abc"), []byte("abc"), true},
		{etcdserverpb.Compare_EQUAL, []byte("abc"), []byte("abd"), false},
		{etcdserverpb.Compare_EQUAL, nil, nil, true},
		{etcdserverpb.Compare_EQUAL, []byte{}, nil, true},
		{etcdserverpb.Compare_NOT_EQUAL, []byte("abc"), []byte("abd"), true},
		{etcdserverpb.Compare_GREATER, []byte("b"), []byte("a"), true},
		{etcdserverpb.Compare_GREATER, []byte("a"), []byte("b"), false},
		{etcdserverpb.Compare_LESS, []byte("a"), []byte("b"), true},
	}

	for _, tc := range tests {
		got := compareBytes(tc.op, tc.got, tc.want)
		assert.Equal(t, tc.expected, got, "op=%v got=%q want=%q", tc.op, tc.got, tc.want)
	}
}

func TestCompareSingle_Version(t *testing.T) {
	t.Parallel()

	kv := &domain.KVPair{Version: 5}

	cmp := &etcdserverpb.Compare{
		Target:      etcdserverpb.Compare_VERSION,
		Result:      etcdserverpb.Compare_EQUAL,
		TargetUnion: &etcdserverpb.Compare_Version{Version: 5},
	}
	assert.True(t, compareSingle(cmp, kv))

	cmp.TargetUnion = &etcdserverpb.Compare_Version{Version: 6}
	assert.False(t, compareSingle(cmp, kv))
}

func TestCompareSingle_CreateRev(t *testing.T) {
	t.Parallel()

	kv := &domain.KVPair{CreateRevision: 10}

	cmp := &etcdserverpb.Compare{
		Target:      etcdserverpb.Compare_CREATE,
		Result:      etcdserverpb.Compare_GREATER,
		TargetUnion: &etcdserverpb.Compare_CreateRevision{CreateRevision: 5},
	}
	assert.True(t, compareSingle(cmp, kv))
}

func TestCompareSingle_ModRev(t *testing.T) {
	t.Parallel()

	kv := &domain.KVPair{ModRevision: 20}

	cmp := &etcdserverpb.Compare{
		Target:      etcdserverpb.Compare_MOD,
		Result:      etcdserverpb.Compare_LESS,
		TargetUnion: &etcdserverpb.Compare_ModRevision{ModRevision: 30},
	}
	assert.True(t, compareSingle(cmp, kv))
}

func TestCompareSingle_Value(t *testing.T) {
	t.Parallel()

	kv := &domain.KVPair{Value: []byte("hello")}

	cmp := &etcdserverpb.Compare{
		Target:      etcdserverpb.Compare_VALUE,
		Result:      etcdserverpb.Compare_EQUAL,
		TargetUnion: &etcdserverpb.Compare_Value{Value: []byte("hello")},
	}
	assert.True(t, compareSingle(cmp, kv))

	cmp.TargetUnion = &etcdserverpb.Compare_Value{Value: []byte("world")}
	assert.False(t, compareSingle(cmp, kv))
}

func TestCompareSingle_NilKV_IsZeroValue(t *testing.T) {
	t.Parallel()

	// When the key doesn't exist, all int targets compare against 0 and value is nil.
	// This is the idiom etcd clients use to assert "key does not exist":
	//   Compare(CreateRevision, "=", 0)
	cmp := &etcdserverpb.Compare{
		Target:      etcdserverpb.Compare_CREATE,
		Result:      etcdserverpb.Compare_EQUAL,
		TargetUnion: &etcdserverpb.Compare_CreateRevision{CreateRevision: 0},
	}
	assert.True(t, compareSingle(cmp, nil), "missing key has CreateRevision=0")

	cmpV := &etcdserverpb.Compare{
		Target:      etcdserverpb.Compare_VERSION,
		Result:      etcdserverpb.Compare_EQUAL,
		TargetUnion: &etcdserverpb.Compare_Version{Version: 0},
	}
	assert.True(t, compareSingle(cmpV, nil), "missing key has Version=0")
}

func TestCompareSingle_UnknownTarget_ReturnsFalse(t *testing.T) {
	t.Parallel()

	cmp := &etcdserverpb.Compare{
		Target: etcdserverpb.Compare_LEASE, // we don't implement lease compares
		Result: etcdserverpb.Compare_EQUAL,
	}
	assert.False(t, compareSingle(cmp, &domain.KVPair{}))
}

func TestSortKVs(t *testing.T) {
	t.Parallel()

	makeKVs := func() []*mvccpb.KeyValue {
		return []*mvccpb.KeyValue{
			{Key: []byte("/b"), CreateRevision: 3, ModRevision: 10, Version: 2, Value: []byte("z")},
			{Key: []byte("/a"), CreateRevision: 1, ModRevision: 5, Version: 5, Value: []byte("y")},
			{Key: []byte("/c"), CreateRevision: 2, ModRevision: 8, Version: 1, Value: []byte("x")},
		}
	}

	t.Run("NONE preserves input", func(t *testing.T) {
		t.Parallel()

		kvs := makeKVs()
		sortKVs(kvs, etcdserverpb.RangeRequest_NONE, etcdserverpb.RangeRequest_KEY)
		assert.Equal(t, []byte("/b"), kvs[0].Key)
		assert.Equal(t, []byte("/a"), kvs[1].Key)
		assert.Equal(t, []byte("/c"), kvs[2].Key)
	})

	t.Run("ASCEND by KEY", func(t *testing.T) {
		t.Parallel()

		kvs := makeKVs()
		sortKVs(kvs, etcdserverpb.RangeRequest_ASCEND, etcdserverpb.RangeRequest_KEY)
		assert.Equal(t, []byte("/a"), kvs[0].Key)
		assert.Equal(t, []byte("/b"), kvs[1].Key)
		assert.Equal(t, []byte("/c"), kvs[2].Key)
	})

	t.Run("DESCEND by KEY", func(t *testing.T) {
		t.Parallel()

		kvs := makeKVs()
		sortKVs(kvs, etcdserverpb.RangeRequest_DESCEND, etcdserverpb.RangeRequest_KEY)
		assert.Equal(t, []byte("/c"), kvs[0].Key)
		assert.Equal(t, []byte("/b"), kvs[1].Key)
		assert.Equal(t, []byte("/a"), kvs[2].Key)
	})

	t.Run("ASCEND by VERSION", func(t *testing.T) {
		t.Parallel()

		kvs := makeKVs()
		sortKVs(kvs, etcdserverpb.RangeRequest_ASCEND, etcdserverpb.RangeRequest_VERSION)
		assert.Equal(t, int64(1), kvs[0].Version)
		assert.Equal(t, int64(2), kvs[1].Version)
		assert.Equal(t, int64(5), kvs[2].Version)
	})

	t.Run("ASCEND by CREATE", func(t *testing.T) {
		t.Parallel()

		kvs := makeKVs()
		sortKVs(kvs, etcdserverpb.RangeRequest_ASCEND, etcdserverpb.RangeRequest_CREATE)
		assert.Equal(t, int64(1), kvs[0].CreateRevision)
		assert.Equal(t, int64(2), kvs[1].CreateRevision)
		assert.Equal(t, int64(3), kvs[2].CreateRevision)
	})

	t.Run("ASCEND by MOD", func(t *testing.T) {
		t.Parallel()

		kvs := makeKVs()
		sortKVs(kvs, etcdserverpb.RangeRequest_ASCEND, etcdserverpb.RangeRequest_MOD)
		assert.Equal(t, int64(5), kvs[0].ModRevision)
		assert.Equal(t, int64(8), kvs[1].ModRevision)
		assert.Equal(t, int64(10), kvs[2].ModRevision)
	})

	t.Run("ASCEND by VALUE", func(t *testing.T) {
		t.Parallel()

		kvs := makeKVs()
		sortKVs(kvs, etcdserverpb.RangeRequest_ASCEND, etcdserverpb.RangeRequest_VALUE)
		assert.Equal(t, []byte("x"), kvs[0].Value)
		assert.Equal(t, []byte("y"), kvs[1].Value)
		assert.Equal(t, []byte("z"), kvs[2].Value)
	})
}

func TestEventToProto_Put(t *testing.T) {
	t.Parallel()

	cfg := &domain.Config{
		Path:           "/foo.json",
		Namespace:      "default",
		Content:        `{"x":1}`,
		CreateRevision: 3,
		Revision:       5,
		Version:        2,
	}
	ev := domain.WatchEvent{
		Type:      domain.EventTypeUpdated,
		Path:      "/foo.json",
		Namespace: "default",
		Revision:  5,
		Config:    cfg,
	}

	got := eventToProto(ev)

	assert.Equal(t, mvccpb.PUT, got.Type)
	assert.Equal(t, []byte("/default/foo.json"), got.Kv.Key)
	assert.Equal(t, []byte(`{"x":1}`), got.Kv.Value)
	assert.Equal(t, int64(3), got.Kv.CreateRevision)
	assert.Equal(t, int64(5), got.Kv.ModRevision)
	assert.Equal(t, int64(2), got.Kv.Version)
}

func TestEventToProto_Delete_CarriesRevision(t *testing.T) {
	t.Parallel()

	// Regression guard for the C3 bug fix: delete events must carry the delete
	// revision in kv.ModRevision, kv.Version must be 0, and kv.Value must be nil.
	ev := domain.WatchEvent{
		Type:      domain.EventTypeDeleted,
		Path:      "/foo.json",
		Namespace: "default",
		Revision:  7,
	}

	got := eventToProto(ev)

	assert.Equal(t, mvccpb.DELETE, got.Type)
	assert.Equal(t, []byte("/default/foo.json"), got.Kv.Key)
	assert.Equal(t, int64(7), got.Kv.ModRevision, "delete must carry delete revision")
	assert.Equal(t, int64(0), got.Kv.Version, "delete resets version to 0")
	assert.Empty(t, got.Kv.Value)
	assert.Equal(t, int64(0), got.Kv.CreateRevision)
}

func TestEventToProto_PutWithNilConfig(t *testing.T) {
	t.Parallel()

	// Degenerate input: PUT-type event without config. Should still produce a
	// well-formed proto (key + ModRevision from ev.Revision) without panicking.
	ev := domain.WatchEvent{
		Type:      domain.EventTypeCreated,
		Path:      "/x",
		Namespace: "ns",
		Revision:  9,
		Config:    nil,
	}

	got := eventToProto(ev)
	assert.Equal(t, mvccpb.PUT, got.Type)
	assert.Equal(t, []byte("/ns/x"), got.Kv.Key)
	assert.Equal(t, int64(9), got.Kv.ModRevision)
}

func TestChangelogToEvent_Put(t *testing.T) {
	t.Parallel()

	e := &domain.ChangelogEntry{
		Revision:  5,
		Type:      domain.EventTypeUpdated,
		Path:      "/foo.json",
		Namespace: "default",
		Version:   2,
	}

	got := changelogToEvent(e, []byte("content"))

	assert.Equal(t, mvccpb.PUT, got.Type)
	assert.Equal(t, []byte("/default/foo.json"), got.Kv.Key)
	assert.Equal(t, int64(5), got.Kv.ModRevision)
	assert.Equal(t, int64(2), got.Kv.Version)
	assert.Equal(t, []byte("content"), got.Kv.Value)
}

func TestChangelogToEvent_Delete(t *testing.T) {
	t.Parallel()

	e := &domain.ChangelogEntry{
		Revision:  9,
		Type:      domain.EventTypeDeleted,
		Path:      "/foo",
		Namespace: "ns",
		Version:   3, // stored version before delete — must be overridden to 0
	}

	got := changelogToEvent(e, []byte("old"))

	assert.Equal(t, mvccpb.DELETE, got.Type)
	assert.Equal(t, int64(9), got.Kv.ModRevision)
	assert.Equal(t, int64(0), got.Kv.Version, "delete forces version=0 per etcd semantics")
	assert.Nil(t, got.Kv.Value)
}

func TestRevisionOfEvent(t *testing.T) {
	t.Parallel()

	// Revision field takes precedence
	ev := domain.WatchEvent{Revision: 42, Config: &domain.Config{Revision: 10}}
	assert.Equal(t, int64(42), revisionOfEvent(ev))

	// Falls back to Config.Revision when Revision is 0
	ev2 := domain.WatchEvent{Config: &domain.Config{Revision: 10}}
	assert.Equal(t, int64(10), revisionOfEvent(ev2))

	// Both zero
	assert.Equal(t, int64(0), revisionOfEvent(domain.WatchEvent{}))

	// Delete-style: no Config, but Revision set
	evDel := domain.WatchEvent{Type: domain.EventTypeDeleted, Revision: 7}
	assert.Equal(t, int64(7), revisionOfEvent(evDel))
}
