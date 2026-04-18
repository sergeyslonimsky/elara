package bbolt

import (
	"bytes"
	"encoding/binary"
)

const (
	keySep = byte(0x00)

	// revisionSize is the byte size of a big-endian uint64 revision.
	revisionSize = 8
)

func configKey(namespace, path string) []byte {
	return []byte(namespace + string(keySep) + path)
}

func parseConfigKey(key []byte) (string, string) {
	before, after, found := bytes.Cut(key, []byte{keySep})
	if !found {
		return "", string(key)
	}

	return string(before), string(after)
}

func configKeyPrefix(namespace, pathPrefix string) []byte {
	if namespace == "" {
		return nil // scan all keys
	}

	return []byte(namespace + string(keySep) + pathPrefix)
}

func historyKey(namespace, path string, revision int64) []byte {
	prefix := historyPrefix(namespace, path)

	return append(prefix, revisionBytes(revision)...)
}

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
