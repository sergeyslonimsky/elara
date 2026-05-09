package pathutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/util/pathutil"
)

func TestNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "/",
		},
		{
			name:  "already normalized",
			input: "/path/to/something",
			want:  "/path/to/something",
		},
		{
			name:  "missing prefix",
			input: "path/to/something",
			want:  "/path/to/something",
		},
		{
			name:  "trailing slash",
			input: "/path/to/something/",
			want:  "/path/to/something",
		},
		{
			name:  "missing prefix and trailing slash",
			input: "path/to/something/",
			want:  "/path/to/something",
		},
		{
			name:  "just slash",
			input: "/",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := pathutil.Normalize(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
