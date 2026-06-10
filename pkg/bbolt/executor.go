package bbolt

import (
	"fmt"
)

// Get fetches the value at key from bucket, JSON-decodes it into T, and
// returns it. Returns ErrNotFound if the key is absent.
//
// Mirrors core/sql.Get[T]: a single Querier-parameterised generic call
// that works identically inside or outside a transaction.
func Get[T any](q Querier, bucket string, key []byte) (T, error) {
	return GetWithCodec[T](q, bucket, key, JSONCodec[T]{})
}

// GetWithCodec is the explicit-codec variant of Get.
func GetWithCodec[T any](q Querier, bucket string, key []byte, codec Codec[T]) (T, error) {
	var zero T

	data := q.Bucket(bucket).Get(key)
	if data == nil {
		return zero, fmt.Errorf("key %q in bucket %q: %w", key, bucket, ErrNotFound)
	}

	var out T
	if err := codec.Unmarshal(data, &out); err != nil {
		return zero, fmt.Errorf("decode %q: %w", key, err)
	}

	return out, nil
}

// Put JSON-encodes value and stores it at key in bucket.
func Put[T any](q Querier, bucket string, key []byte, value T) error {
	return PutWithCodec(q, bucket, key, value, JSONCodec[T]{})
}

// PutWithCodec is the explicit-codec variant of Put.
func PutWithCodec[T any](q Querier, bucket string, key []byte, value T, codec Codec[T]) error {
	data, err := codec.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode %q: %w", key, err)
	}

	if err := q.Bucket(bucket).Put(key, data); err != nil {
		return fmt.Errorf("put %q: %w", key, err)
	}

	return nil
}

// Delete removes key from bucket. Returns nil if the key did not exist.
func Delete(q Querier, bucket string, key []byte) error {
	if err := q.Bucket(bucket).Delete(key); err != nil {
		return fmt.Errorf("delete %q: %w", key, err)
	}

	return nil
}

// Exists reports whether key is present in bucket.
func Exists(q Querier, bucket string, key []byte) bool {
	return q.Bucket(bucket).Get(key) != nil
}

// List decodes every value in bucket into T and returns them in key order.
// Iteration stops on the first decode error.
func List[T any](q Querier, bucket string) ([]T, error) {
	return ListWithCodec[T](q, bucket, JSONCodec[T]{})
}

// ListWithCodec is the explicit-codec variant of List.
func ListWithCodec[T any](q Querier, bucket string, codec Codec[T]) ([]T, error) {
	var out []T

	err := q.Bucket(bucket).ForEach(func(_, v []byte) error {
		var item T
		if err := codec.Unmarshal(v, &item); err != nil {
			return fmt.Errorf("decode: %w", err)
		}

		out = append(out, item)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list bucket %q: %w", bucket, err)
	}

	return out, nil
}

// Scan decodes every value whose key starts with prefix and returns them in
// key order. Requires a cursor, so the Querier MUST come from WithTx /
// WithReadTx — calling Scan on an autoQuerier returns an empty result.
func Scan[T any](q Querier, bucket string, prefix []byte) ([]T, error) {
	return ScanWithCodec[T](q, bucket, prefix, JSONCodec[T]{})
}

// ScanWithCodec is the explicit-codec variant of Scan.
func ScanWithCodec[T any](
	q Querier,
	bucket string,
	prefix []byte,
	codec Codec[T],
) ([]T, error) {
	var out []T

	c := q.Bucket(bucket).Cursor()
	for k, v := c.Seek(prefix); k != nil && hasPrefix(k, prefix); k, v = c.Next() {
		var item T
		if err := codec.Unmarshal(v, &item); err != nil {
			return nil, fmt.Errorf("decode %q: %w", k, err)
		}

		out = append(out, item)
	}

	return out, nil
}

func hasPrefix(s, prefix []byte) bool {
	if len(s) < len(prefix) {
		return false
	}

	for i := range prefix {
		if s[i] != prefix[i] {
			return false
		}
	}

	return true
}
