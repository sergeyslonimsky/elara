package maputil

import "maps"

// Clone creates a shallow copy of a map.
//
//nolint:ireturn // false positive for generic type M
func Clone[M ~map[K]V, K comparable, V any](src M) M {
	if src == nil {
		return make(M)
	}

	dst := make(M, len(src))
	maps.Copy(dst, src)

	return dst
}
