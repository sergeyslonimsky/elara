package sliceutil_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/util/sliceutil"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		old, in     []string
		wantAdded   []string
		wantRemoved []string
	}{
		{
			name:        "all added",
			old:         nil,
			in:          []string{"a", "b"},
			wantAdded:   []string{"a", "b"},
			wantRemoved: nil,
		},
		{
			name:        "all removed",
			old:         []string{"a", "b"},
			in:          nil,
			wantAdded:   nil,
			wantRemoved: []string{"a", "b"},
		},
		{
			name:        "mixed",
			old:         []string{"a", "b", "c"},
			in:          []string{"b", "c", "d"},
			wantAdded:   []string{"d"},
			wantRemoved: []string{"a"},
		},
		{
			name:        "identical",
			old:         []string{"a", "b"},
			in:          []string{"a", "b"},
			wantAdded:   nil,
			wantRemoved: nil,
		},
		{
			name:        "duplicates collapsed",
			old:         []string{"a", "a"},
			in:          []string{"a", "b", "b"},
			wantAdded:   []string{"b"},
			wantRemoved: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			added, removed := sliceutil.Diff(tt.old, tt.in)
			sort.Strings(added)
			sort.Strings(removed)

			assert.Equal(t, tt.wantAdded, added)
			assert.Equal(t, tt.wantRemoved, removed)
		})
	}
}

func TestUnion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b []int
		want []int
	}{
		{name: "disjoint", a: []int{1, 2}, b: []int{3, 4}, want: []int{1, 2, 3, 4}},
		{name: "overlap", a: []int{1, 2, 3}, b: []int{2, 3, 4}, want: []int{1, 2, 3, 4}},
		{name: "duplicates within", a: []int{1, 1, 2}, b: []int{2, 2}, want: []int{1, 2}},
		{name: "empty inputs", a: nil, b: nil, want: []int{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := sliceutil.Union(tt.a, tt.b)
			sort.Ints(got)

			assert.Equal(t, tt.want, got)
		})
	}
}
