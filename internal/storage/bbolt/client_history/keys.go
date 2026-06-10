package client_history

import (
	"encoding/binary"
	"time"
)

// timeKeySize is the byte length of the time portion of a history key
// (big-endian unix-nano timestamp).
const timeKeySize = 8

// timeKey encodes a time.Time as an 8-byte big-endian unix-nano value so
// bbolt's lexicographic cursor order matches chronological order (oldest
// first). Same-nano collisions are disambiguated at write-time by appending
// the connection ID to the key.
func timeKey(t time.Time) []byte {
	k := make([]byte, timeKeySize)
	binary.BigEndian.PutUint64(k, uint64(t.UnixNano()))

	return k
}

// bytesLess returns true if a < b under big-endian unsigned comparison.
// Length mismatch falls back to length comparison — defensive for callers
// that pass full keys (8 bytes + ID suffix) against an 8-byte cutoff.
func bytesLess(a, b []byte) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}

	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}

	return false
}
