package bbolt

import (
	"encoding/json"
	"fmt"
)

// Codec encodes and decodes values stored under bucket keys. Repositories
// pass a Codec to generic helpers (Get[T], Put[T], List[T]) when they need
// a non-JSON wire format. The zero-value default across the package is
// JSONCodec — use it implicitly via the unsuffixed helpers, or explicitly
// via the WithCodec-suffixed variants for non-JSON encodings.
type Codec[T any] interface {
	Marshal(v T) ([]byte, error)
	Unmarshal(data []byte, v *T) error
}

// JSONCodec is the default Codec. It uses encoding/json — predictable,
// debuggable, and matches the historical on-disk format of elara's repos.
type JSONCodec[T any] struct{}

func (JSONCodec[T]) Marshal(v T) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("json marshal: %w", err)
	}

	return data, nil
}

func (JSONCodec[T]) Unmarshal(data []byte, v *T) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("json unmarshal: %w", err)
	}

	return nil
}
