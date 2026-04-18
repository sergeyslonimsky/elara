package etcdv3

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		key           []byte
		wantNamespace string
		wantPath      string
		wantOK        bool
	}{
		{
			name:          "simple two-segment",
			key:           []byte("/default/foo.json"),
			wantNamespace: "default",
			wantPath:      "/foo.json",
			wantOK:        true,
		},
		{
			name:          "nested path",
			key:           []byte("/prod/services/api.yaml"),
			wantNamespace: "prod",
			wantPath:      "/services/api.yaml",
			wantOK:        true,
		},
		{
			name:          "deeply nested path",
			key:           []byte("/ns/a/b/c/d/file.txt"),
			wantNamespace: "ns",
			wantPath:      "/a/b/c/d/file.txt",
			wantOK:        true,
		},
		{
			name:          "namespace only — no trailing slash",
			key:           []byte("/default"),
			wantNamespace: "default",
			wantPath:      "/",
			wantOK:        true,
		},
		{
			name:          "namespace only — trailing slash",
			key:           []byte("/default/"),
			wantNamespace: "default",
			wantPath:      "/",
			wantOK:        true,
		},
		{
			name:   "empty key",
			key:    []byte{},
			wantOK: false,
		},
		{
			name:   "missing leading slash",
			key:    []byte("default/foo"),
			wantOK: false,
		},
		{
			name:   "only slash — empty namespace",
			key:    []byte("/"),
			wantOK: false,
		},
		{
			name:   "double slash — empty namespace",
			key:    []byte("//foo"),
			wantOK: false,
		},
		{
			name:          "path with special chars",
			key:           []byte("/ns/path with spaces.json"),
			wantNamespace: "ns",
			wantPath:      "/path with spaces.json",
			wantOK:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ns, path, ok := SplitKey(tc.key)
			assert.Equal(t, tc.wantOK, ok, "ok mismatch")

			if tc.wantOK {
				assert.Equal(t, tc.wantNamespace, ns, "namespace")
				assert.Equal(t, tc.wantPath, path, "path")
			}
		})
	}
}

func TestJoinKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		namespace string
		path      string
		want      []byte
	}{
		{
			name:      "simple",
			namespace: "default",
			path:      "/foo.json",
			want:      []byte("/default/foo.json"),
		},
		{
			name:      "nested",
			namespace: "prod",
			path:      "/services/api.yaml",
			want:      []byte("/prod/services/api.yaml"),
		},
		{
			name:      "empty path → root",
			namespace: "ns",
			path:      "",
			want:      []byte("/ns/"),
		},
		{
			name:      "path without leading slash gets one",
			namespace: "ns",
			path:      "foo",
			want:      []byte("/ns/foo"),
		},
		{
			name:      "root path",
			namespace: "default",
			path:      "/",
			want:      []byte("/default/"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := JoinKey(tc.namespace, tc.path)
			assert.True(t, bytes.Equal(tc.want, got), "want=%q got=%q", tc.want, got)
		})
	}
}

func TestSplitKey_JoinKey_Roundtrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		namespace string
		path      string
	}{
		{"default", "/foo.json"},
		{"prod", "/services/api/config.yaml"},
		{"ns", "/a/b/c"},
	}

	for _, tc := range cases {
		encoded := JoinKey(tc.namespace, tc.path)
		ns, path, ok := SplitKey(encoded)
		assert.True(t, ok, "SplitKey failed for %q", encoded)
		assert.Equal(t, tc.namespace, ns)
		assert.Equal(t, tc.path, path)
	}
}

func TestSplitRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		key        []byte
		rangeEnd   []byte
		wantSNS    string
		wantSPath  string
		wantENS    string
		wantEPath  string
		wantOK     bool
		wantSingle bool // expected semantic: endNS=="" && endPath==""
		wantAll    bool // expected semantic: endNS=="\x00"
	}{
		{
			name:       "single key",
			key:        []byte("/default/foo"),
			rangeEnd:   nil,
			wantSNS:    "default",
			wantSPath:  "/foo",
			wantOK:     true,
			wantSingle: true,
		},
		{
			name:      "scan all (range_end=\\0)",
			key:       []byte("/default/"),
			rangeEnd:  []byte{0},
			wantSNS:   "default",
			wantSPath: "/",
			wantENS:   "\x00",
			wantOK:    true,
			wantAll:   true,
		},
		{
			name:      "prefix range within namespace",
			key:       []byte("/default/"),
			rangeEnd:  []byte("/default0"), // prefix end byte 0x2f+1 = 0x30
			wantSNS:   "default",
			wantSPath: "/",
			wantENS:   "default0",
			wantEPath: "/",
			wantOK:    true,
		},
		{
			name:      "explicit bounded range",
			key:       []byte("/ns/a"),
			rangeEnd:  []byte("/ns/z"),
			wantSNS:   "ns",
			wantSPath: "/a",
			wantENS:   "ns",
			wantEPath: "/z",
			wantOK:    true,
		},
		{
			name:     "invalid key",
			key:      []byte("bad"),
			rangeEnd: nil,
			wantOK:   false,
		},
		{
			name:     "invalid range_end",
			key:      []byte("/ns/x"),
			rangeEnd: []byte("bad"),
			wantOK:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sns, spath, ens, epath, ok := SplitRange(tc.key, tc.rangeEnd)
			assert.Equal(t, tc.wantOK, ok)

			if !tc.wantOK {
				return
			}

			assert.Equal(t, tc.wantSNS, sns)
			assert.Equal(t, tc.wantSPath, spath)
			assert.Equal(t, tc.wantENS, ens)
			assert.Equal(t, tc.wantEPath, epath)

			single := ens == "" && epath == ""
			all := ens == "\x00"
			assert.Equal(t, tc.wantSingle, single, "singleKey flag")
			assert.Equal(t, tc.wantAll, all, "scanAll flag")
		})
	}
}

func TestCommonPathPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a, b string
		want string
	}{
		{"/services/api/v1", "/services/api/v2", "/services/api/v"},
		{"/foo", "/bar", "/"},
		{"/", "/foo", "/"},
		{"/same", "/same", "/same"},
		{"", "/foo", ""},
		{"/foo", "", ""},
		{"/a/b/c", "/a/b/d", "/a/b/"},
		{"/a/b", "/a/b/c", "/a/b"},
	}

	for _, tc := range tests {
		got := commonPathPrefix(tc.a, tc.b)
		assert.Equal(t, tc.want, got, "commonPathPrefix(%q, %q)", tc.a, tc.b)
	}
}
