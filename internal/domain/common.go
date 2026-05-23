package domain

// AuthInfo carries identity attributes extracted from the auth context and
// passed explicitly to usecases (the usecase layer does not consult the
// request context for claims).
type AuthInfo struct {
	Email      string
	Name       string
	Namespaces []string
	Role       string
}

// SortParams holds sorting parameters for list operations.
type SortParams struct {
	Field string // "name", "modified"
	Desc  bool   // true = descending
}

// KVPair is a single key-value entry in etcd semantics. It lives here so
// that the etcd handler can depend on it without importing the bbolt
// adapter.
type KVPair struct {
	Namespace      string
	Path           string
	Value          []byte
	CreateRevision int64
	ModRevision    int64
	Version        int64
}
