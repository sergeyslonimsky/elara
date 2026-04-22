package etcdv3

import (
	"bytes"
	"strings"
)

// SplitKey decodes an etcd key of the form "/{namespace}/{path}" into (namespace, path).
// The path always starts with a leading "/". Returns ok=false if the key cannot be parsed.
//
// Examples:
//
//	/prod/foo.json             → namespace="prod",    path="/foo.json"
//	/prod/services/api.yaml    → namespace="prod",    path="/services/api.yaml"
//	/prod                      → namespace="prod",    path="/"          (namespace prefix, no config path)
//	foo                        → ok=false (missing leading /)
func SplitKey(etcdKey []byte) (string, string, bool) {
	if len(etcdKey) == 0 || etcdKey[0] != '/' {
		return "", "", false
	}

	rest := etcdKey[1:]

	idx := bytes.IndexByte(rest, '/')
	if idx < 0 {
		// No second slash — whole thing after `/` is namespace with empty path.
		namespace := string(rest)
		if namespace == "" {
			return "", "", false
		}

		return namespace, "/", true
	}

	namespace := string(rest[:idx])
	if namespace == "" {
		return "", "", false
	}

	path := string(rest[idx:]) // keeps leading "/"

	return namespace, path, true
}

// JoinKey encodes (namespace, path) into an etcd key "/{namespace}{path}" where path
// must start with "/". If path is empty it is treated as "/".
func JoinKey(namespace, path string) []byte {
	if path == "" || !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	key := make([]byte, 0, 1+len(namespace)+len(path))
	key = append(key, '/')
	key = append(key, namespace...)
	key = append(key, path...)

	return key
}

// SplitRange decodes a (key, range_end) pair from etcd into storage-level
// (startNS, startPath, endNS, endPath, singleKey).
//
// etcd semantics:
//   - range_end empty → single key at `key`
//   - range_end == []byte{0} → range over all keys >= key
//   - otherwise → range [key, range_end)
//
// This function translates those cases into our namespace/path encoding.
// The caller should treat the returned (endNS, endPath) = ("\x00", "") as
// "scan everything >= start" (matching bbolt.ConfigRepo.RangeQuery semantics).
func SplitRange(key, rangeEnd []byte) (string, string, string, string, bool) {
	startNS, startPath, parsed := SplitKey(key)
	if !parsed {
		return "", "", "", "", false
	}

	switch {
	case len(rangeEnd) == 0:
		// single key
		return startNS, startPath, "", "", true

	case bytes.Equal(rangeEnd, []byte{0}):
		// scan all keys >= start
		return startNS, startPath, "\x00", "", true

	default:
		eNS, ePath, eOk := SplitKey(rangeEnd)
		if !eOk {
			return "", "", "", "", false
		}

		return startNS, startPath, eNS, ePath, true
	}
}
