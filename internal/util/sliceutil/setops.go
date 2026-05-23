package sliceutil

// Diff returns (added, removed): elements in incoming but not in old, and
// elements in old but not in incoming. Result order is non-deterministic
// because it is computed via map iteration.
func Diff[T comparable](old, incoming []T) ([]T, []T) {
	oldMap := make(map[T]struct{}, len(old))
	for _, v := range old {
		oldMap[v] = struct{}{}
	}

	newMap := make(map[T]struct{}, len(incoming))
	for _, v := range incoming {
		newMap[v] = struct{}{}
	}

	var added, removed []T
	for v := range newMap {
		if _, ok := oldMap[v]; !ok {
			added = append(added, v)
		}
	}

	for v := range oldMap {
		if _, ok := newMap[v]; !ok {
			removed = append(removed, v)
		}
	}

	return added, removed
}

// Union returns the set union of a and b as a slice (each element once).
// Result order is non-deterministic because it is computed via map
// iteration.
func Union[T comparable](a, b []T) []T {
	m := make(map[T]struct{}, len(a)+len(b))
	for _, v := range a {
		m[v] = struct{}{}
	}
	for _, v := range b {
		m[v] = struct{}{}
	}

	out := make([]T, 0, len(m))
	for v := range m {
		out = append(out, v)
	}

	return out
}
