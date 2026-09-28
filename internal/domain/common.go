package domain

// AuthInfo carries identity attributes extracted from the auth context and
// passed explicitly to usecases (the usecase layer does not consult the
// request context for claims).
type AuthInfo struct {
	UserID     string
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
	// Lease is the lease this key is attached to, or zero when it is not
	// attached to one. Zero is how the etcd wire protocol spells "no lease", so
	// the zero value needs no special case.
	Lease int64
}

// KeyRef addresses a single key without carrying its value. It is the unit a
// lease attaches to, and the unit a bulk delete takes — revoking a lease has to
// remove an arbitrary set of keys, which is not expressible as a range.
type KeyRef struct {
	Namespace string
	Path      string
}
