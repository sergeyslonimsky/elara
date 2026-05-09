package maputil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/util/maputil"
)

func TestClone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  map[string]int
		want map[string]int
	}{
		{
			name: "nil map",
			src:  nil,
			want: map[string]int{},
		},
		{
			name: "empty map",
			src:  map[string]int{},
			want: map[string]int{},
		},
		{
			name: "populated map",
			src:  map[string]int{"a": 1, "b": 2},
			want: map[string]int{"a": 1, "b": 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := maputil.Clone(tt.src)
			assert.Equal(t, tt.want, got)

			if len(tt.src) > 0 {
				got["mutated"] = 99
				assert.NotContains(t, tt.src, "mutated")
			}
		})
	}
}
