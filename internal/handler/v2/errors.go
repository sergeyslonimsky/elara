package v2

import (
	"errors"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func toConnectError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, domain.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, domain.ErrAlreadyExists):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, domain.ErrConflict):
		return connect.NewError(connect.CodeAborted, err)
	case errors.Is(err, domain.ErrInvalidFormat):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case domain.IsValidationError(err):
		return connect.NewError(connect.CodeInvalidArgument, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
