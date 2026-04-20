package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_normalizeContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		content string
		want    string
	}{
		{
			name:    "json valid",
			path:    "app.json",
			content: `{"b":2,"a":1}`,
			want:    "{\n  \"a\": 1,\n  \"b\": 2\n}",
		},
		{
			name:    "json key order stable",
			path:    "app.json",
			content: `{"z":3,"a":1,"m":2}`,
			want:    "{\n  \"a\": 1,\n  \"m\": 2,\n  \"z\": 3\n}",
		},
		{
			name:    "json invalid fallback",
			path:    "app.json",
			content: `{not valid json`,
			want:    `{not valid json`,
		},
		{
			name:    "yaml valid",
			path:    "config.yaml",
			content: "foo:   bar\nbaz:   qux\n",
			want:    "baz: qux\nfoo: bar\n",
		},
		{
			name:    "yml extension",
			path:    "config.yml",
			content: "key:   value\n",
			want:    "key: value\n",
		},
		{
			name:    "yaml invalid fallback",
			path:    "config.yaml",
			content: "foo: [unclosed",
			want:    "foo: [unclosed",
		},
		{
			name:    "empty content",
			path:    "app.json",
			content: "",
			want:    "",
		},
		{
			name:    "no extension raw",
			path:    "Makefile",
			content: "build:\n\tgo build",
			want:    "build:\n\tgo build",
		},
		{
			name:    "unknown extension raw",
			path:    "config.toml",
			content: "[section]\nkey = \"val\"",
			want:    "[section]\nkey = \"val\"",
		},
		{
			name:    "txt extension raw",
			path:    "notes.txt",
			content: "hello world",
			want:    "hello world",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeContent(tc.path, tc.content)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
