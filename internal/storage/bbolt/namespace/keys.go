package namespace

import (
	"encoding/binary"
)

const (
	keySep   = byte(0x00)
	revBytes = 8
)

// historyPrefix builds the (namespace, path) prefix used as the lock_history
// bucket key prefix. The full key appends the sequence revision (8 BE bytes).
func historyPrefix(namespace, path string) []byte {
	return []byte(namespace + string(keySep) + path + string(keySep))
}

func revisionBytes(rev int64) []byte {
	b := make([]byte, revBytes)
	binary.BigEndian.PutUint64(b, uint64(rev))

	return b
}

func parseRevision(b []byte) int64 {
	if len(b) < revBytes {
		return 0
	}

	return int64(binary.BigEndian.Uint64(b))
}

// configKeyPrefix returns the prefix for scanning configs by namespace in the
// `meta` bucket. Identical encoding to bbolt.configKey ("<ns>\x00<path>") —
// keep in sync with config repo until CountConfigs moves there.
func configKeyPrefix(namespace string) []byte {
	if namespace == "" {
		return nil
	}

	return []byte(namespace + string(keySep))
}
