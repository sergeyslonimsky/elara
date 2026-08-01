package archive_test

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/util/archive"
)

func TestWrapInZip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
		data     []byte
	}{
		{
			name:     "wrap simple text",
			filename: "test.txt",
			data:     []byte("hello world"),
		},
		{
			name:     "empty data",
			filename: "empty.bin",
			data:     []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := archive.WrapInZip(tt.filename, tt.data)
			require.NoError(t, err)

			zr, err := zip.NewReader(bytes.NewReader(got), int64(len(got)))
			require.NoError(t, err)
			require.Len(t, zr.File, 1)
			assert.Equal(t, tt.filename, zr.File[0].Name)

			assert.True(t, archive.IsZIP(got))
		})
	}
}

func TestUnzipIfNeeded(t *testing.T) {
	t.Parallel()

	emptyZipBuf := new(bytes.Buffer)
	zw := zip.NewWriter(emptyZipBuf)
	_, _ = zw.Create("empty_dir/")
	require.NoError(t, zw.Close())
	emptyZip := emptyZipBuf.Bytes()

	tests := []struct {
		name    string
		data    []byte
		errIs   error
		wantErr string
		want    []byte
	}{
		{
			name: "not a zip file",
			data: []byte("just plain text"),
			want: []byte("just plain text"),
		},
		{
			name: "empty byte slice",
			data: []byte{},
			want: []byte{},
		},
		{
			name: "valid zip file",
			data: func() []byte {
				b, _ := archive.WrapInZip("file.txt", []byte("zipped content"))

				return b
			}(),
			want: []byte("zipped content"),
		},
		{
			name:  "empty zip file",
			data:  emptyZip,
			errIs: archive.ErrEmptyZip,
		},
		{
			name:    "invalid zip data",
			data:    []byte{0x50, 0x4B, 0x03, 0x04, 0x00, 0x00, 0x00},
			wantErr: "not a valid zip file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := archive.UnzipIfNeeded(tt.data)

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

func TestUnzipIfNeeded_CorruptedEntry(t *testing.T) {
	t.Parallel()

	// Build a two-entry zip by hand: a directory entry (skipped by the
	// IsDir continue branch) followed by a real file entry. This lets us
	// corrupt the SECOND entry's local file header without touching the
	// archive's first four bytes, which IsZIP inspects to decide whether
	// to attempt unzipping at all.
	buildTwoEntryZip := func(t *testing.T) []byte {
		t.Helper()

		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		_, err := zw.Create("empty_dir/")
		require.NoError(t, err)

		fw, err := zw.Create("file.txt")
		require.NoError(t, err)
		_, err = fw.Write([]byte("payload contents for corruption tests"))
		require.NoError(t, err)

		require.NoError(t, zw.Close())

		return buf.Bytes()
	}

	tests := []struct {
		name    string
		corrupt func(t *testing.T, data []byte) []byte
		wantErr string
	}{
		{
			name: "corrupted local file header signature causes open error",
			corrupt: func(t *testing.T, data []byte) []byte {
				t.Helper()

				sig := []byte{0x50, 0x4B, 0x03, 0x04}
				// Skip the first occurrence (offset 0, the directory entry -
				// corrupting it would also break IsZIP's signature check).
				firstIdx := bytes.Index(data, sig)
				require.GreaterOrEqual(t, firstIdx, 0)

				secondIdx := bytes.Index(data[firstIdx+1:], sig)
				require.GreaterOrEqual(t, secondIdx, 0)
				secondIdx += firstIdx + 1

				out := append([]byte(nil), data...)
				out[secondIdx] ^= 0xFF

				return out
			},
			wantErr: "open zip file file.txt",
		},
		{
			name: "corrupted compressed payload causes read error",
			corrupt: func(t *testing.T, data []byte) []byte {
				t.Helper()

				sig := []byte{0x50, 0x4B, 0x03, 0x04}
				firstIdx := bytes.Index(data, sig)
				require.GreaterOrEqual(t, firstIdx, 0)

				secondIdx := bytes.Index(data[firstIdx+1:], sig)
				require.GreaterOrEqual(t, secondIdx, 0)
				secondIdx += firstIdx + 1

				// Local file header is 30 fixed bytes + filename ("file.txt",
				// 8 bytes) + extra field (0 bytes here). Flip the first byte
				// of the compressed data stream right after the header so
				// the deflate decoder chokes on it, without touching the
				// header itself or any trailing data descriptor.
				payloadStart := secondIdx + 30 + len("file.txt")
				require.Less(t, payloadStart, len(data))

				out := append([]byte(nil), data...)
				out[payloadStart] ^= 0xFF

				return out
			},
			wantErr: "read zip file file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := tt.corrupt(t, buildTwoEntryZip(t))
			require.True(t, archive.IsZIP(data))

			_, err := archive.UnzipIfNeeded(data)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestIsZIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "valid zip signature",
			data: []byte{0x50, 0x4B, 0x03, 0x04, 0x01, 0x02},
			want: true,
		},
		{
			name: "too short",
			data: []byte{0x50, 0x4B, 0x03},
			want: false,
		},
		{
			name: "invalid signature",
			data: []byte{0x50, 0x4B, 0x03, 0x05},
			want: false,
		},
		{
			name: "empty",
			data: []byte{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := archive.IsZIP(tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}
