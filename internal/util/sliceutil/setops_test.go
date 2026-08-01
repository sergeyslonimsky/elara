package sliceutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/util/sliceutil"
)

func TestToSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []int
		want map[int]struct{}
	}{
		{
			name: "multiple elements",
			in:   []int{1, 2, 3},
			want: map[int]struct{}{1: {}, 2: {}, 3: {}},
		},
		{
			name: "duplicate elements collapse",
			in:   []int{1, 1, 2},
			want: map[int]struct{}{1: {}, 2: {}},
		},
		{
			name: "empty slice",
			in:   []int{},
			want: map[int]struct{}{},
		},
		{
			name: "nil slice",
			in:   nil,
			want: map[int]struct{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sliceutil.ToSet(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNotIn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    []string
		set  map[string]struct{}
		want []string
	}{
		{
			name: "some elements absent from set",
			s:    []string{"a", "b", "c"},
			set:  map[string]struct{}{"b": {}},
			want: []string{"a", "c"},
		},
		{
			name: "all elements present in set",
			s:    []string{"a", "b"},
			set:  map[string]struct{}{"a": {}, "b": {}},
			want: []string{},
		},
		{
			name: "empty set returns all elements preserving order",
			s:    []string{"a", "b"},
			set:  map[string]struct{}{},
			want: []string{"a", "b"},
		},
		{
			name: "empty slice",
			s:    []string{},
			set:  map[string]struct{}{"a": {}},
			want: []string{},
		},
		{
			name: "nil slice",
			s:    nil,
			set:  map[string]struct{}{"a": {}},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sliceutil.NotIn(tt.s, tt.set)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    []string
		set  map[string]struct{}
		want []string
	}{
		{
			name: "some elements present in set",
			s:    []string{"a", "b", "c"},
			set:  map[string]struct{}{"b": {}},
			want: []string{"b"},
		},
		{
			name: "no elements present in set",
			s:    []string{"a", "b"},
			set:  map[string]struct{}{"z": {}},
			want: []string{},
		},
		{
			name: "empty slice",
			s:    []string{},
			set:  map[string]struct{}{"a": {}},
			want: []string{},
		},
		{
			name: "nil slice",
			s:    nil,
			set:  map[string]struct{}{"a": {}},
			want: []string{},
		},
		{
			name: "empty set",
			s:    []string{"a", "b"},
			set:  map[string]struct{}{},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sliceutil.In(tt.s, tt.set)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFirstOverlap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		a      []int
		b      []int
		want   int
		wantOk bool
	}{
		{
			name:   "overlap found",
			a:      []int{1, 2, 3},
			b:      []int{4, 2, 5},
			want:   2,
			wantOk: true,
		},
		{
			name:   "disjoint slices",
			a:      []int{1, 2},
			b:      []int{3, 4},
			want:   0,
			wantOk: false,
		},
		{
			name:   "empty a",
			a:      []int{},
			b:      []int{1, 2},
			want:   0,
			wantOk: false,
		},
		{
			name:   "empty b",
			a:      []int{1, 2},
			b:      []int{},
			want:   0,
			wantOk: false,
		},
		{
			name:   "both nil",
			a:      nil,
			b:      nil,
			want:   0,
			wantOk: false,
		},
		{
			name:   "first element of b matches",
			a:      []int{5, 6},
			b:      []int{5, 7},
			want:   5,
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := sliceutil.FirstOverlap(tt.a, tt.b)
			assert.Equal(t, tt.wantOk, ok)
			if tt.wantOk {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestComposePost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current []string
		added   []string
		removed []string
		want    []string
	}{
		{
			name:    "add and remove",
			current: []string{"a", "b", "c"},
			added:   []string{"d"},
			removed: []string{"b"},
			want:    []string{"a", "c", "d"},
		},
		{
			name:    "no changes",
			current: []string{"a", "b"},
			added:   nil,
			removed: nil,
			want:    []string{"a", "b"},
		},
		{
			name:    "remove all current",
			current: []string{"a", "b"},
			added:   nil,
			removed: []string{"a", "b"},
			want:    []string{},
		},
		{
			name:    "add to empty current",
			current: []string{},
			added:   []string{"x", "y"},
			removed: []string{},
			want:    []string{"x", "y"},
		},
		{
			name:    "removed item not present is no-op",
			current: []string{"a"},
			added:   []string{},
			removed: []string{"z"},
			want:    []string{"a"},
		},
		{
			name:    "all nil",
			current: nil,
			added:   nil,
			removed: nil,
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sliceutil.ComposePost(tt.current, tt.added, tt.removed)
			assert.Equal(t, tt.want, got)
		})
	}
}
