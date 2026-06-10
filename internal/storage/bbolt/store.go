package bbolt

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

//nolint:gochecknoglobals // bucket list initialized once at startup, read-only thereafter
var buckets = [][]byte{
	[]byte("content"),
	[]byte("meta"),
	[]byte("namespaces"),
	[]byte("changelog"),
	[]byte("history"),
	[]byte("client_history"),
	[]byte("sys"),
	[]byte("lock_history"),
	[]byte("lock_changelog"),
	[]byte("schemas"),
	[]byte("webhooks"),
	[]byte("auth_users"),
	[]byte("users_by_identity"),
	[]byte("users_by_email"),
	[]byte("auth_groups"),
	[]byte("auth_tokens"),
	[]byte("auth_token_by_id"),
	[]byte("auth_policy"),
	[]byte("sessions"),
	[]byte("sessions_by_user"),
	[]byte("session_events"),
	[]byte("session_events_by_session"),
	[]byte("session_events_by_user"),
}

const (
	bucketSysName  = "sys"
	sysRevisionKey = "revision"
	sysSchemaKey   = "schema"

	schemaVersion uint64 = 1

	revisionSize = 8
)

type Store struct {
	db *bolt.DB
}

func Open(path string) (*Store, error) {
	dir := filepath.Dir(path)
	const dirPerm = 0o755

	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	const filePerm = 0o600

	db, err := bolt.Open(path, filePerm, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bbolt database: %w", err)
	}

	if err := initBuckets(db); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("init buckets: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close bbolt database: %w", err)
	}

	return nil
}

// Shutdown closes the underlying bbolt database. Implements lifecycle.Resource.
func (s *Store) Shutdown(_ context.Context) error {
	return s.Close()
}

func (s *Store) DB() *bolt.DB {
	return s.db
}

func initBuckets(db *bolt.DB) error {
	err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range buckets {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return fmt.Errorf("create bucket %s: %w", name, err)
			}
		}

		sys := tx.Bucket([]byte(bucketSysName))

		if sys.Get([]byte(sysRevisionKey)) == nil {
			if err := sys.Put([]byte(sysRevisionKey), revisionBytes(0)); err != nil {
				return fmt.Errorf("init revision: %w", err)
			}
		}

		if sys.Get([]byte(sysSchemaKey)) == nil {
			if err := sys.Put([]byte(sysSchemaKey), revisionBytes(int64(schemaVersion))); err != nil {
				return fmt.Errorf("init schema version: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("init buckets: %w", err)
	}

	return nil
}

func revisionBytes(rev int64) []byte {
	b := make([]byte, revisionSize)
	binary.BigEndian.PutUint64(b, uint64(rev))

	return b
}
