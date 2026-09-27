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

	assert.Equal(t, []byte("/default/foo.json"), got.GetKey())
	assert.Equal(t, []byte(`{"x":1}`), got.GetValue())
	assert.Equal(t, int64(10), got.GetCreateRevision())
	assert.Equal(t, int64(15), got.GetModRevision())
	assert.Equal(t, int64(3), got.GetVersion())
}

func TestNewHeader(t *testing.T) {
	t.Parallel()

	h := newHeader(42)

	assert.Equal(t, int64(42), h.GetRevision())
	assert.Equal(t, clusterID, h.GetClusterId())
	assert.Equal(t, memberID, h.GetMemberId())
	assert.Equal(t, raftTerm, h.GetRaftTerm())
}

// Compare semantics (compareInt64 / compareBytes / compareKV) moved with the Txn
// orchestration into usecase/config — see service_txn_internal_test.go there.

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
		assert.Equal(t, []byte("/b"), kvs[0].GetKey())
		assert.Equal(t, []byte("/a"), kvs[1].GetKey())
		assert.Equal(t, []byte("/c"), kvs[2].GetKey())
	})

	t.Run("ASCEND by KEY", func(t *testing.T) {
		t.Parallel()

		kvs := makeKVs()
		sortKVs(kvs, etcdserverpb.RangeRequest_ASCEND, etcdserverpb.RangeRequest_KEY)
		assert.Equal(t, []byte("/a"), kvs[0].GetKey())
		assert.Equal(t, []byte("/b"), kvs[1].GetKey())
		assert.Equal(t, []byte("/c"), kvs[2].GetKey())
	})

	t.Run("DESCEND by KEY", func(t *testing.T) {
		t.Parallel()

		kvs := makeKVs()
		sortKVs(kvs, etcdserverpb.RangeRequest_DESCEND, etcdserverpb.RangeRequest_KEY)
		assert.Equal(t, []byte("/c"), kvs[0].GetKey())
		assert.Equal(t, []byte("/b"), kvs[1].GetKey())
		assert.Equal(t, []byte("/a"), kvs[2].GetKey())
	})

	t.Run("ASCEND by VERSION", func(t *testing.T) {
		t.Parallel()

		kvs := makeKVs()
		sortKVs(kvs, etcdserverpb.RangeRequest_ASCEND, etcdserverpb.RangeRequest_VERSION)
		assert.Equal(t, int64(1), kvs[0].GetVersion())
		assert.Equal(t, int64(2), kvs[1].GetVersion())
		assert.Equal(t, int64(5), kvs[2].GetVersion())
	})

	t.Run("ASCEND by CREATE", func(t *testing.T) {
		t.Parallel()

		kvs := makeKVs()
		sortKVs(kvs, etcdserverpb.RangeRequest_ASCEND, etcdserverpb.RangeRequest_CREATE)
		assert.Equal(t, int64(1), kvs[0].GetCreateRevision())
		assert.Equal(t, int64(2), kvs[1].GetCreateRevision())
		assert.Equal(t, int64(3), kvs[2].GetCreateRevision())
	})

	t.Run("ASCEND by MOD", func(t *testing.T) {
		t.Parallel()

		kvs := makeKVs()
		sortKVs(kvs, etcdserverpb.RangeRequest_ASCEND, etcdserverpb.RangeRequest_MOD)
		assert.Equal(t, int64(5), kvs[0].GetModRevision())
		assert.Equal(t, int64(8), kvs[1].GetModRevision())
		assert.Equal(t, int64(10), kvs[2].GetModRevision())
	})

	t.Run("ASCEND by VALUE", func(t *testing.T) {
		t.Parallel()

		kvs := makeKVs()
		sortKVs(kvs, etcdserverpb.RangeRequest_ASCEND, etcdserverpb.RangeRequest_VALUE)
		assert.Equal(t, []byte("x"), kvs[0].GetValue())
		assert.Equal(t, []byte("y"), kvs[1].GetValue())
		assert.Equal(t, []byte("z"), kvs[2].GetValue())
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

	assert.Equal(t, mvccpb.PUT, got.GetType())
	assert.Equal(t, []byte("/default/foo.json"), got.GetKv().GetKey())
	assert.Equal(t, []byte(`{"x":1}`), got.GetKv().GetValue())
	assert.Equal(t, int64(3), got.GetKv().GetCreateRevision())
	assert.Equal(t, int64(5), got.GetKv().GetModRevision())
	assert.Equal(t, int64(2), got.GetKv().GetVersion())
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

	assert.Equal(t, mvccpb.DELETE, got.GetType())
	assert.Equal(t, []byte("/default/foo.json"), got.GetKv().GetKey())
	assert.Equal(t, int64(7), got.GetKv().GetModRevision(), "delete must carry delete revision")
	assert.Equal(t, int64(0), got.GetKv().GetVersion(), "delete resets version to 0")
	assert.Empty(t, got.GetKv().GetValue())
	assert.Equal(t, int64(0), got.GetKv().GetCreateRevision())
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
	assert.Equal(t, mvccpb.PUT, got.GetType())
	assert.Equal(t, []byte("/ns/x"), got.GetKv().GetKey())
	assert.Equal(t, int64(9), got.GetKv().GetModRevision())
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

	assert.Equal(t, mvccpb.PUT, got.GetType())
	assert.Equal(t, []byte("/default/foo.json"), got.GetKv().GetKey())
	assert.Equal(t, int64(5), got.GetKv().GetModRevision())
	assert.Equal(t, int64(2), got.GetKv().GetVersion())
	assert.Equal(t, []byte("content"), got.GetKv().GetValue())
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

	assert.Equal(t, mvccpb.DELETE, got.GetType())
	assert.Equal(t, int64(9), got.GetKv().GetModRevision())
	assert.Equal(t, int64(0), got.GetKv().GetVersion(), "delete forces version=0 per etcd semantics")
	assert.Nil(t, got.GetKv().GetValue())
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
