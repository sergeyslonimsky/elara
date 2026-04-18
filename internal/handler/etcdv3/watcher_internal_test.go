package etcdv3

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// makeWatcher constructs a watcher whose filter fields are derived from
// (key, rangeEnd) using the same logic as createWatcher. This lets us test the
// matchers in isolation with realistic input.
func makeWatcher(t *testing.T, key, rangeEnd []byte) *watcher {
	t.Helper()

	startNS, startPath, endNS, endPath, ok := SplitRange(key, rangeEnd)
	if !ok {
		t.Fatalf("invalid key/rangeEnd: %q / %q", key, rangeEnd)
	}

	scanAll := endNS == "\x00"
	singleKey := endNS == "" && endPath == ""

	w := &watcher{
		start:     key,
		end:       rangeEnd,
		scanAll:   scanAll,
		singleKey: singleKey,
	}

	switch {
	case scanAll:
		// no-op: namespace and pathPrefix left empty
	case singleKey:
		w.namespace = startNS
		w.exactPath = startPath
		w.pathPrefix = startPath
	case startNS == endNS:
		w.namespace = startNS
		w.pathPrefix = commonPathPrefix(startPath, endPath)
	default:
		// cross-namespace: leave both empty so matchesKey checks via start/end bounds.
	}

	return w
}

func TestWatcher_MatchesKey_SingleKey(t *testing.T) {
	t.Parallel()

	w := makeWatcher(t, []byte("/default/foo.json"), nil)

	assert.True(t, w.matchesKey("default", "/foo.json"), "exact hit")
	assert.False(t, w.matchesKey("default", "/foo.json.bak"), "suffix must not match")
	assert.False(t, w.matchesKey("default", "/foo.jsonx"), "different path")
	assert.False(t, w.matchesKey("default", "/other.json"), "different path")
	assert.False(t, w.matchesKey("prod", "/foo.json"), "different namespace")
}

func TestWatcher_MatchesKey_Prefix_SameNamespace(t *testing.T) {
	t.Parallel()

	// clientv3 WithPrefix("/default/") increments the last byte: rangeEnd = "/default0".
	w := makeWatcher(t, []byte("/default/"), []byte("/default0"))

	assert.True(t, w.matchesKey("default", "/foo.json"))
	assert.True(t, w.matchesKey("default", "/sub/nested.yaml"))
	assert.True(t, w.matchesKey("default", "/"), "root path within namespace")
	assert.False(t, w.matchesKey("prod", "/foo.json"), "different namespace")
	assert.False(t, w.matchesKey("other", "/x"), "lexically outside range")
}

func TestWatcher_MatchesKey_ScanAll(t *testing.T) {
	t.Parallel()

	// etcd convention: key="\0", range_end="\0" → all keys
	// But SplitRange expects a valid leading-slash key; clientv3 actually sends
	// key="\x00" for "all", which isn't a valid namespace-encoded key in our scheme.
	// We test the supported shape: any key + range_end="\0".
	w := makeWatcher(t, []byte("/a/"), []byte{0})

	assert.True(t, w.matchesKey("default", "/any"))
	assert.True(t, w.matchesKey("prod", "/services/api"))
	assert.True(t, w.matchesKey("", "/x"), "empty namespace still matches scan-all")
}

func TestWatcher_MatchesKey_CrossNamespace(t *testing.T) {
	t.Parallel()

	// Range spanning namespaces: /a/x to /z/y
	w := makeWatcher(t, []byte("/a/x"), []byte("/z/y"))

	// In range
	assert.True(t, w.matchesKey("a", "/x"), "start key")
	assert.True(t, w.matchesKey("a", "/y"), "past start within ns a")
	assert.True(t, w.matchesKey("m", "/anything"), "middle namespace")
	assert.True(t, w.matchesKey("z", "/a"), "just before end in ns z")

	// Outside
	assert.False(t, w.matchesKey("a", "/w"), "before start")
	assert.False(t, w.matchesKey("z", "/y"), "at end (exclusive)")
	assert.False(t, w.matchesKey("z", "/z"), "past end")
}

func TestWatcher_MatchesKey_BoundedRangeSameNamespace(t *testing.T) {
	t.Parallel()

	// Range /ns/b to /ns/m — strictly within one namespace.
	w := makeWatcher(t, []byte("/ns/b"), []byte("/ns/m"))

	assert.True(t, w.matchesKey("ns", "/b"), "start (inclusive)")
	assert.True(t, w.matchesKey("ns", "/c/nested"))
	assert.True(t, w.matchesKey("ns", "/l"))
	assert.False(t, w.matchesKey("ns", "/a"), "before start")
	assert.False(t, w.matchesKey("ns", "/m"), "at end (exclusive)")
	assert.False(t, w.matchesKey("ns", "/z"), "past end")
	assert.False(t, w.matchesKey("other", "/c"), "different namespace outside range")
}

func TestWatcher_MatchesEvent_MatchesChangelog_Delegation(t *testing.T) {
	t.Parallel()

	// Ensure the two wrappers delegate correctly.
	w := makeWatcher(t, []byte("/default/"), []byte("/default0"))

	ev := domain.WatchEvent{Namespace: "default", Path: "/foo"}
	assert.True(t, w.matchesEvent(ev))

	entry := &domain.ChangelogEntry{Namespace: "default", Path: "/foo"}
	assert.True(t, w.matchesChangelog(entry))

	evMiss := domain.WatchEvent{Namespace: "prod", Path: "/foo"}
	assert.False(t, w.matchesEvent(evMiss))
}

func TestWatcher_MatchesKey_EdgeCases(t *testing.T) {
	t.Parallel()

	// Single-key watch at namespace-root: "/default"
	w := makeWatcher(t, []byte("/default"), nil)
	assert.True(t, w.matchesKey("default", "/"), "singleKey exact match at root")
	assert.False(t, w.matchesKey("default", "/foo"))

	// Prefix range on root of namespace — should cover every path in that ns
	w2 := makeWatcher(t, []byte("/default/"), []byte("/default0"))
	assert.True(t, w2.matchesKey("default", "/"))
	assert.True(t, w2.matchesKey("default", "/a/deep/nested/thing"))
}

func TestWatcher_Matches_UsesRevisionSemantics(t *testing.T) {
	t.Parallel()

	// Regression guard: a past bug used HasPrefix for single-key, causing /foo to match /foo.json.
	w := makeWatcher(t, []byte("/ns/foo"), nil)
	assert.False(t, w.matchesKey("ns", "/foo.json"), "must not prefix-match for single-key")
	assert.False(t, w.matchesKey("ns", "/foobar"))
	assert.True(t, w.matchesKey("ns", "/foo"))
}

func TestWatcher_MatchesEvent_IgnoresTimestampAndConfig(t *testing.T) {
	t.Parallel()

	// matchesEvent must only consider namespace + path; other fields are irrelevant.
	w := makeWatcher(t, []byte("/default/foo"), nil)

	ev := domain.WatchEvent{
		Type:      domain.EventTypeDeleted,
		Namespace: "default",
		Path:      "/foo",
		Revision:  42,
		Config:    nil,
		Timestamp: time.Unix(0, 0),
	}
	assert.True(t, w.matchesEvent(ev))
}
