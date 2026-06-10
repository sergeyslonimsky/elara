package storage

import (
	"errors"
)

// Storage-layer sentinels are the vocabulary repositories use to signal
// generic existence outcomes. They are NOT alias-equal to the domain
// sentinels — `errors.Is(storage.ErrResourceNotFound, domain.ErrNotFound)`
// is false. The boundary between layers (usecase or service) is responsible
// for remapping: catch the storage sentinel and rewrap with the domain one
// before returning to handlers.
//
// Storage may depend on domain (entities live there); the reverse is
// forbidden by depguard. Hence storage cannot wrap domain sentinels at the
// repo edge — that would invert the dependency.
var (
	ErrResourceAlreadyExists = errors.New("resource already exists")
	ErrResourceNotFound      = errors.New("resource not found")
)
