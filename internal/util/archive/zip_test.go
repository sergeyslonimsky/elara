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
