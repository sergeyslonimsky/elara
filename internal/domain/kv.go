package domain

// KVPair is a single key-value entry in etcd semantics.
// It lives in the domain package so that the etcd handler can depend on it
// without importing the bbolt adapter.
type KVPair struct {
	Namespace      string
	Path           string
	Value          []byte
	CreateRevision int64
	ModRevision    int64
	Version        int64
}
