package bbolt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
)

func TestOpen(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    func(t *testing.T) string
		wantErr string
	}{
		{
			name: "success creates data directory and db file",
			path: func(t *testing.T) string {
				t.Helper()

				return filepath.Join(t.TempDir(), "nested", "dir", "elara.db")
			},
		},
		{
			name: "invalid path returns error",
			path: func(t *testing.T) string {
				t.Helper()
				// A file used as a directory component is not a valid path to create.
				base := t.TempDir()
				blocker := filepath.Join(base, "blocker")
				require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))

				return filepath.Join(blocker, "sub", "elara.db")
			},
			wantErr: "create data directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := tt.path(t)

			store, err := bbolt.Open(path)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = store.Close()
			})

			assert.NotNil(t, store.DB())
			assert.FileExists(t, path)
		})
	}
}

func TestStore_ReOpen_InitBucketsIdempotent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "elara.db")

	store1, err := bbolt.Open(path)
	require.NoError(t, err)
	require.NoError(t, store1.Close())

	store2, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = store2.Close()
	})

	assert.NotNil(t, store2.DB())
}

func TestStore_Close(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "elara.db")

	store, err := bbolt.Open(path)
	require.NoError(t, err)

	err = store.Close()
	require.NoError(t, err)
}

func TestStore_Shutdown(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "elara.db")

	store, err := bbolt.Open(path)
	require.NoError(t, err)

	err = store.Shutdown(t.Context())
	require.NoError(t, err)
}
