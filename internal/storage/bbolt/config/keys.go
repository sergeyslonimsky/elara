package config

import (
	"bytes"
	"encoding/binary"
)

const (
	keySep = byte(0x00)
	// revisionSize is the byte size of a big-endian uint64 revision.
	revisionSize = 8
)

// configKey encodes (namespace, path) as "<ns>\x00<path>" — the key shape used
// by the `meta` and `content` buckets.
func configKey(namespace, path string) []byte {
	return []byte(namespace + string(keySep) + path)
}

// parseConfigKey is the inverse of configKey.
func parseConfigKey(key []byte) (string, string) {
	before, after, found := bytes.Cut(key, []byte{keySep})
	if !found {
		return "", string(key)
	}

	return string(before), string(after)
}

// configKeyPrefix returns the prefix for scanning configs by namespace and
// path prefix in the `meta` / `content` buckets. Returns nil when namespace
// is empty (caller scans all keys and applies its own filtering).
func configKeyPrefix(namespace, pathPrefix string) []byte {
	if namespace == "" {
		return nil
	}

	return []byte(namespace + string(keySep) + pathPrefix)
}

// historyKey encodes (namespace, path, revision) as the lock_history /
// history bucket key.
func historyKey(namespace, path string, revision int64) []byte {
	prefix := historyPrefix(namespace, path)

	return append(prefix, revisionBytes(revision)...)
}

// historyPrefix encodes (namespace, path) as the key prefix used to scan
// history / lock_history entries for a single config.
func historyPrefix(namespace, path string) []byte {
	return []byte(namespace + string(keySep) + path + string(keySep))
}

func revisionBytes(rev int64) []byte {
	b := make([]byte, revisionSize)
	binary.BigEndian.PutUint64(b, uint64(rev))

	return b
}

func parseRevision(b []byte) int64 {
	if len(b) < revisionSize {
		return 0
	}

	return int64(binary.BigEndian.Uint64(b))
}
