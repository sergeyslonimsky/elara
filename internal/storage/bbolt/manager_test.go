package bbolt_test

import (
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	"github.com/sergeyslonimsky/elara/internal/storage/storagetest"
)

func TestManager_Contract(t *testing.T) {
	t.Parallel()

	storagetest.RunManagerContract(t, func(t *testing.T) storage.Manager {
		t.Helper()
		path := filepath.Join(t.TempDir(), "test.db")
		db, err := bolt.Open(path, 0o600, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = db.Close()
		})

		return bbolt.NewManager(db)
	})
}
