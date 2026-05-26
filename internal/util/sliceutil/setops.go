package sliceutil

// ToSet returns a map keyed by the slice's elements, useful for O(1)
// membership tests in explicit-delta flows.
func ToSet[T comparable](s []T) map[T]struct{} {
	out := make(map[T]struct{}, len(s))
	for _, v := range s {
		out[v] = struct{}{}
	}

	return out
}

// NotIn returns the elements of s that are NOT in set, preserving order.
// Used to compute "effective add": items in caller's add-list that aren't
// already present.
func NotIn[T comparable](s []T, set map[T]struct{}) []T {
	out := make([]T, 0, len(s))
	for _, v := range s {
		if _, ok := set[v]; !ok {
			out = append(out, v)
		}
	}

	return out
}

// In returns the elements of s that ARE in set, preserving order.
// Used to compute "effective remove": items in caller's remove-list that
// are actually present.
func In[T comparable](s []T, set map[T]struct{}) []T {
	out := make([]T, 0, len(s))
	for _, v := range s {
		if _, ok := set[v]; ok {
			out = append(out, v)
		}
	}

	return out
}

// FirstOverlap reports the first element appearing in both a and b, or
// the zero value with ok=false when the two slices are disjoint. Used to
// reject inputs where the same item is requested for both add and remove.
func FirstOverlap[T comparable](a, b []T) (T, bool) {
	var zero T
	if len(a) == 0 || len(b) == 0 {
		return zero, false
	}
	set := ToSet(a)
	for _, v := range b {
		if _, ok := set[v]; ok {
			return v, true
		}
	}

	return zero, false
}

// ComposePost returns the post-delta state computed locally as
// (current − removed) ∪ added. Order: kept items first (in their original
// order), then added (in their original order). Used to render response
// payloads without a second enforcer round-trip after applying a delta.
func ComposePost[T comparable](current, added, removed []T) []T {
	removeSet := ToSet(removed)
	out := make([]T, 0, len(current)+len(added))
	for _, v := range current {
		if _, dropped := removeSet[v]; !dropped {
			out = append(out, v)
		}
	}
	out = append(out, added...)

	return out
}
