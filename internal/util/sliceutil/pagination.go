package sliceutil

// Paginate returns the slice of items for the given offset and limit.
func Paginate[T any](items []T, offset, limit int) []T {
	total := len(items)
	if offset >= total {
		return nil
	}

	end := min(offset+limit, total)

	return items[offset:end]
}
