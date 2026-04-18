package domain

// SortParams holds sorting parameters for list operations.
type SortParams struct {
	Field string // "name", "modified"
	Desc  bool   // true = descending
}
