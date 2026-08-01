package bbolt_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

// testItem is the value type used across executor/codec tests.
type testItem struct {
	ID    string `json:"id"`
	Value int    `json:"value"`
}

// failCodec is a bbolt.Codec[T] that always errors, used to exercise the
// encode/decode error branches of the generic executor helpers.
type failCodec[T any] struct{}

func (failCodec[T]) Marshal(T) ([]byte, error) {
	return nil, errors.New("marshal fail")
}

func (failCodec[T]) Unmarshal([]byte, *T) error {
	return errors.New("unmarshal fail")
}

// newTestDB opens a real bbolt.DB backed by a temp file and creates the given
// top-level buckets (if any). The DB is closed automatically via t.Cleanup.
func newTestDB(t *testing.T, buckets ...string) *bolt.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")

	db, err := bolt.Open(path, 0o600, nil)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = db.Close()
	})

	if len(buckets) > 0 {
		err = db.Update(func(tx *bolt.Tx) error {
			for _, name := range buckets {
				if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
					return err
				}
			}

			return nil
		})
		require.NoError(t, err)
	}

	return db
}

// putRaw writes value directly into bucket/key, bypassing any codec. Used to
// seed malformed data for decode-error test cases.
func putRaw(t *testing.T, db *bolt.DB, bucket, key, value string) {
	t.Helper()

	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))

		return b.Put([]byte(key), []byte(value))
	})
	require.NoError(t, err)
}
