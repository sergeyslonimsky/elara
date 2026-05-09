package sliceutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/util/sliceutil"
)

func TestPaginate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		items  []int
		offset int
		limit  int
		want   []int
	}{
		{
			name:   "first page",
			items:  []int{1, 2, 3, 4, 5},
			offset: 0,
			limit:  2,
			want:   []int{1, 2},
		},
		{
			name:   "second page",
			items:  []int{1, 2, 3, 4, 5},
			offset: 2,
			limit:  2,
			want:   []int{3, 4},
		},
		{
			name:   "last page incomplete",
			items:  []int{1, 2, 3, 4, 5},
			offset: 4,
			limit:  2,
			want:   []int{5},
		},
		{
			name:   "offset exceeds length",
			items:  []int{1, 2, 3, 4, 5},
			offset: 5,
			limit:  2,
			want:   nil,
		},
		{
			name:   "offset far beyond length",
			items:  []int{1, 2, 3},
			offset: 10,
			limit:  5,
			want:   nil,
		},
		{
			name:   "limit larger than length",
			items:  []int{1, 2, 3},
			offset: 0,
			limit:  10,
			want:   []int{1, 2, 3},
		},
		{
			name:   "empty slice",
			items:  []int{},
			offset: 0,
			limit:  5,
			want:   nil,
		},
		{
			name:   "nil slice",
			items:  nil,
			offset: 0,
			limit:  5,
			want:   nil,
		},
		{
			name:   "zero limit",
			items:  []int{1, 2, 3},
			offset: 0,
			limit:  0,
			want:   []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sliceutil.Paginate(tt.items, tt.offset, tt.limit)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPaginate_Strings(t *testing.T) {
	t.Parallel()

	items := []string{"a", "b", "c"}
	got := sliceutil.Paginate(items, 1, 1)
	assert.Equal(t, []string{"b"}, got)
}
