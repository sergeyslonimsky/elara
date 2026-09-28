package lease

import (
	"bytes"
	"encoding/binary"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

const (
	bucketLeases      = "leases"
	bucketLeaseKeys   = "lease_keys"
	bucketLeaseExpiry = "lease_expiry"

	keySep = byte(0x00)

	// idSize is the byte size of a big-endian lease ID or unix-nano timestamp.
	idSize = 8
)

// leaseKey encodes a lease ID as the primary bucket key.
//
// Big-endian so the bucket is ordered by ID rather than by little-endian byte
// soup — bbolt keys sort lexicographically, and an ordered primary bucket costs
// nothing here while making a range scan possible later.
func leaseKey(id int64) []byte {
	return idBytes(id)
}

// leaseKeysKey encodes (leaseID, namespace, path) as the lease_keys index key:
// <be(id)> + sep + <ns> + sep + <path>. The same 0x00 separator the config
// buckets use (see storage/bbolt/config/keys.go).
func leaseKeysKey(id int64, ref domain.KeyRef) []byte {
	key := leaseKeysPrefix(id)
	key = append(key, []byte(ref.Namespace)...)
	key = append(key, keySep)

	return append(key, []byte(ref.Path)...)
}

// leaseKeysPrefix is the prefix every lease_keys entry of one lease shares.
func leaseKeysPrefix(id int64) []byte {
	return append(idBytes(id), keySep)
}

// parseLeaseKeysKey is the inverse of leaseKeysKey for one lease's entries: it
// takes the portion after the lease prefix and splits it back into namespace and
// path.
func parseLeaseKeysKey(suffix []byte) domain.KeyRef {
	ns, path, found := bytes.Cut(suffix, []byte{keySep})
	if !found {
		return domain.KeyRef{Namespace: "", Path: string(suffix)}
	}

	return domain.KeyRef{Namespace: string(ns), Path: string(path)}
}

// expiryKey encodes (expiresAt, leaseID) as the lease_expiry index key:
// <be(unixNano)> + <be(id)>.
//
// Timestamp first, so the bucket is sorted by due time and the sweep can walk
// from the first key and stop at the first lease that is not due yet. The ID is
// appended because two leases can share an expiry instant and a bucket key must
// be unique.
func expiryKey(expiresAt time.Time, id int64) []byte {
	return append(nanoBytes(expiresAt), idBytes(id)...)
}

// parseExpiryKey extracts the lease ID from a lease_expiry key. Returns false
// for a malformed key rather than panicking on a short slice: an index is
// derived data, and a corrupt entry should be skippable.
func parseExpiryKey(key []byte) (int64, bool) {
	if len(key) != 2*idSize {
		return 0, false
	}

	return int64(binary.BigEndian.Uint64(key[idSize:])), true
}

// idBytes encodes an int64 big-endian.
func idBytes(id int64) []byte {
	b := make([]byte, idSize)
	binary.BigEndian.PutUint64(b, uint64(id))

	return b
}

// nanoBytes encodes a timestamp as big-endian unix nanoseconds.
//
// A pre-epoch time is clamped to zero: its UnixNano is negative, and a negative
// value reinterpreted as uint64 would sort *after* every real timestamp, hiding
// the entry from the sweep forever. Clamping puts such a record at the front of
// the index instead, where the next sweep revokes it — the right outcome for a
// lease whose expiry is nonsense.
func nanoBytes(t time.Time) []byte {
	nanos := max(t.UnixNano(), 0)

	b := make([]byte, idSize)
	binary.BigEndian.PutUint64(b, uint64(nanos))

	return b
}
