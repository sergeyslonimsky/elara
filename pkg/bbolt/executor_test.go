package bbolt_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/pkg/bbolt"
)

func TestGet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T, db *bolt.DB)
		key     string
		errIs   error
		wantErr string
		want    testItem
	}{
		{
			name: "success",
			setup: func(t *testing.T, db *bolt.DB) {
				t.Helper()
				putRaw(t, db, "items", "a", `{"id":"a","value":1}`)
			},
			key:  "a",
			want: testItem{ID: "a", Value: 1},
		},
		{
			name:  "not found",
			setup: func(*testing.T, *bolt.DB) {},
			key:   "missing",
			errIs: bbolt.ErrNotFound,
		},
		{
			name: "decode error",
			setup: func(t *testing.T, db *bolt.DB) {
				t.Helper()
				putRaw(t, db, "items", "bad", "not-json")
			},
			key:     "bad",
			wantErr: `decode "bad": json unmarshal:`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := newTestDB(t, "items")
			tt.setup(t, db)
			mgr := bbolt.NewManager(db)

			got, err := bbolt.Get[testItem](mgr.GetQuerier(t.Context()), "items", []byte(tt.key))

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetWithCodec_CustomCodecDecodeError(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	putRaw(t, db, "items", "a", "1")
	mgr := bbolt.NewManager(db)

	_, err := bbolt.GetWithCodec[testItem](mgr.GetQuerier(t.Context()), "items", []byte("a"), failCodec[testItem]{})
	require.ErrorContains(t, err, `decode "a": unmarshal fail`)
}

func TestPut(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		bucketExists bool
		key          string
		value        testItem
		codec        bbolt.Codec[testItem]
		errIs        error
		wantErr      string
	}{
		{
			name:         "success",
			bucketExists: true,
			key:          "a",
			value:        testItem{ID: "a", Value: 1},
			codec:        bbolt.JSONCodec[testItem]{},
		},
		{
			name:         "bucket missing",
			bucketExists: false,
			key:          "a",
			value:        testItem{ID: "a"},
			codec:        bbolt.JSONCodec[testItem]{},
			errIs:        bbolt.ErrBucketNotFound,
		},
		{
			name:         "encode error",
			bucketExists: true,
			key:          "a",
			value:        testItem{ID: "a"},
			codec:        failCodec[testItem]{},
			wantErr:      `encode "a": marshal fail`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var db *bolt.DB
			if tt.bucketExists {
				db = newTestDB(t, "items")
			} else {
				db = newTestDB(t)
			}
			mgr := bbolt.NewManager(db)

			err := bbolt.PutWithCodec(mgr.GetQuerier(t.Context()), "items", []byte(tt.key), tt.value, tt.codec)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)

			got, getErr := bbolt.Get[testItem](mgr.GetQuerier(t.Context()), "items", []byte(tt.key))
			require.NoError(t, getErr)
			assert.Equal(t, tt.value, got)
		})
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		bucketExists bool
		seed         bool
		key          string
		errIs        error
	}{
		{name: "success", bucketExists: true, seed: true, key: "a"},
		{name: "missing key is noop", bucketExists: true, seed: false, key: "missing"},
		{name: "bucket missing", bucketExists: false, key: "a", errIs: bbolt.ErrBucketNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var db *bolt.DB
			if tt.bucketExists {
				db = newTestDB(t, "items")
			} else {
				db = newTestDB(t)
			}
			mgr := bbolt.NewManager(db)

			if tt.seed {
				require.NoError(
					t,
					bbolt.Put(mgr.GetQuerier(t.Context()), "items", []byte(tt.key), testItem{ID: tt.key}),
				)
			}

			err := bbolt.Delete(mgr.GetQuerier(t.Context()), "items", []byte(tt.key))

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			require.NoError(t, err)
			assert.False(t, bbolt.Exists(mgr.GetQuerier(t.Context()), "items", []byte(tt.key)))
		})
	}
}

func TestExists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		seed bool
		key  string
		want bool
	}{
		{name: "present", seed: true, key: "a", want: true},
		{name: "absent", seed: false, key: "missing", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := newTestDB(t, "items")
			mgr := bbolt.NewManager(db)

			if tt.seed {
				require.NoError(
					t,
					bbolt.Put(mgr.GetQuerier(t.Context()), "items", []byte(tt.key), testItem{ID: tt.key}),
				)
			}

			assert.Equal(t, tt.want, bbolt.Exists(mgr.GetQuerier(t.Context()), "items", []byte(tt.key)))
		})
	}
}

func TestList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T, db *bolt.DB)
		wantErr string
		want    []testItem
	}{
		{
			name: "success in key order",
			setup: func(t *testing.T, db *bolt.DB) {
				t.Helper()
				putRaw(t, db, "items", "a", `{"id":"a","value":1}`)
				putRaw(t, db, "items", "b", `{"id":"b","value":2}`)
			},
			want: []testItem{{ID: "a", Value: 1}, {ID: "b", Value: 2}},
		},
		{
			name:  "empty bucket",
			setup: func(*testing.T, *bolt.DB) {},
			want:  nil,
		},
		{
			name: "decode error aborts",
			setup: func(t *testing.T, db *bolt.DB) {
				t.Helper()
				putRaw(t, db, "items", "a", `{"id":"a","value":1}`)
				putRaw(t, db, "items", "b", "not-json")
			},
			wantErr: `list bucket "items": bbolt foreach: decode:`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := newTestDB(t, "items")
			tt.setup(t, db)
			mgr := bbolt.NewManager(db)

			got, err := bbolt.List[testItem](mgr.GetQuerier(t.Context()), "items")

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestList_BucketMissing(t *testing.T) {
	t.Parallel()

	db := newTestDB(t) // no buckets
	mgr := bbolt.NewManager(db)

	_, err := bbolt.List[testItem](mgr.GetQuerier(t.Context()), "items")
	require.ErrorIs(t, err, bbolt.ErrBucketNotFound)
}

func TestScan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		useTx   bool
		prefix  string
		wantErr string
		want    []testItem
	}{
		{
			name:   "matches by prefix in key order",
			useTx:  true,
			prefix: "a/",
			want:   []testItem{{ID: "a/1", Value: 1}, {ID: "a/2", Value: 2}},
		},
		{
			name:   "no matches",
			useTx:  true,
			prefix: "z/",
			want:   nil,
		},
		{
			name:   "auto querier cursor unsupported returns empty",
			useTx:  false,
			prefix: "a/",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := newTestDB(t, "items")
			putRaw(t, db, "items", "a/1", `{"id":"a/1","value":1}`)
			putRaw(t, db, "items", "a/2", `{"id":"a/2","value":2}`)
			putRaw(t, db, "items", "b/1", `{"id":"b/1","value":3}`)
			mgr := bbolt.NewManager(db)

			var (
				got []testItem
				err error
			)

			if tt.useTx {
				err = mgr.WithTx(t.Context(), func(ctx context.Context) error {
					var scanErr error
					got, scanErr = bbolt.Scan[testItem](mgr.GetQuerier(ctx), "items", []byte(tt.prefix))

					return scanErr
				})
			} else {
				got, err = bbolt.Scan[testItem](mgr.GetQuerier(t.Context()), "items", []byte(tt.prefix))
			}

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestScanWithCodec_DecodeError(t *testing.T) {
	t.Parallel()

	db := newTestDB(t, "items")
	putRaw(t, db, "items", "a/1", "irrelevant")
	mgr := bbolt.NewManager(db)

	err := mgr.WithTx(t.Context(), func(ctx context.Context) error {
		_, scanErr := bbolt.ScanWithCodec[testItem](mgr.GetQuerier(ctx), "items", []byte("a/"), failCodec[testItem]{})

		return scanErr
	})
	require.ErrorContains(t, err, `decode "a/1": unmarshal fail`)
}
