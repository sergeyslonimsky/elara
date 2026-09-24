package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestParseFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    domain.Format
		wantErr bool
	}{
		{
			name:  "json",
			input: "json",
			want:  domain.FormatJSON,
		},
		{
			name:  "yaml",
			input: "yaml",
			want:  domain.FormatYAML,
		},
		{
			// yml is an accepted spelling of the same format, not a format of
			// its own — both must land on FormatYAML so that a config stored
			// as .yml is not treated as a different type downstream.
			name:  "yml is an alias for yaml",
			input: "yml",
			want:  domain.FormatYAML,
		},
		{
			// "other" is a parseable value, not a fallback: ParseFormat is
			// strict everywhere else, and an unrecognized string is an error
			// rather than silently becoming FormatOther.
			name:  "other",
			input: "other",
			want:  domain.FormatOther,
		},
		{
			name:    "unsupported format",
			input:   "toml",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "matching is case sensitive",
			input:   "JSON",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.ParseFormat(tt.input)

			if tt.wantErr {
				require.ErrorIs(t, err, domain.ErrInvalidFormat)
				assert.Empty(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetectFormatFromPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want domain.Format
	}{
		{
			name: "json extension",
			path: "/app/config.json",
			want: domain.FormatJSON,
		},
		{
			name: "yaml extension",
			path: "/app/config.yaml",
			want: domain.FormatYAML,
		},
		{
			name: "yml extension",
			path: "/app/config.yml",
			want: domain.FormatYAML,
		},
		{
			name: "unrecognized extension",
			path: "/app/config.toml",
			want: domain.FormatOther,
		},
		{
			name: "no extension",
			path: "/app/config",
			want: domain.FormatOther,
		},
		{
			// Detection is by suffix, so a format name appearing anywhere but
			// at the end must not win.
			name: "extension name inside the path",
			path: "/json/config.txt",
			want: domain.FormatOther,
		},
		{
			name: "empty path",
			path: "",
			want: domain.FormatOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, domain.DetectFormatFromPath(tt.path))
		})
	}
}

func TestValidatePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{
			name: "valid path",
			path: "/app/config.json",
		},
		{
			name: "valid nested path",
			path: "/a/b/c/d",
		},
		{
			name: "single segment",
			path: "/app",
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: "path is required",
		},
		{
			name:    "missing leading slash",
			path:    "app/config.json",
			wantErr: "path must start with /",
		},
		{
			name:    "contains double slash",
			path:    "/app//config.json",
			wantErr: "path must not contain //",
		},
		{
			name:    "trailing slash",
			path:    "/app/config/",
			wantErr: "path must not end with /",
		},
		{
			// "/" passes the prefix check and contains no "//", so it falls
			// through to the trailing-slash rule. Root is therefore not a
			// valid config path.
			name:    "root",
			path:    "/",
			wantErr: "path must not end with /",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := domain.ValidatePath(tt.path)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.True(t, domain.IsValidationError(err))

				return
			}

			require.NoError(t, err)
		})
	}
}
