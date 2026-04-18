package v2

import (
	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

const (
	// maxPaginationLimit caps the number of items a single list call can return.
	// Chosen to bound memory/bandwidth for a single response even if the client
	// requests something absurd.
	maxPaginationLimit = 10_000

	// maxPaginationOffset caps the number of items a client can skip. Offsets
	// beyond this point are never useful in practice and typically indicate an
	// application defect or abusive caller.
	maxPaginationOffset = 1_000_000
)

// normalizeLimit validates and clamps a client-supplied page limit.
//
// Returns an InvalidArgument ConnectRPC error when the limit is negative or
// larger than maxPaginationLimit. Zero is treated as "unspecified" and caller
// decides the default.
func normalizeLimit(limit int32) (int, error) {
	if limit < 0 {
		return 0, connect.NewError(
			connect.CodeInvalidArgument,
			domain.NewValidationError("limit", "must be non-negative"),
		)
	}

	if limit > maxPaginationLimit {
		return 0, connect.NewError(
			connect.CodeInvalidArgument,
			domain.NewValidationError("limit", "exceeds maximum allowed"),
		)
	}

	return int(limit), nil
}

// normalizeOffset validates a client-supplied page offset.
func normalizeOffset(offset int32) (int, error) {
	if offset < 0 {
		return 0, connect.NewError(
			connect.CodeInvalidArgument,
			domain.NewValidationError("offset", "must be non-negative"),
		)
	}

	if offset > maxPaginationOffset {
		return 0, connect.NewError(
			connect.CodeInvalidArgument,
			domain.NewValidationError("offset", "exceeds maximum allowed"),
		)
	}

	return int(offset), nil
}
