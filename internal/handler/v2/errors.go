package v2

import (
	"errors"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/domain"
	configv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1"
)

//nolint:cyclop // error-mapping switch is intentionally exhaustive; splitting would obscure the mapping
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
	case errors.Is(err, domain.ErrLocked):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, domain.ErrInvalidFormat):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, domain.ErrUnauthorized):
		return connect.NewError(connect.CodeUnauthenticated, err)
	case errors.Is(err, domain.ErrForbidden):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, domain.ErrInvalidToken):
		return connect.NewError(connect.CodeUnauthenticated, err)
	case domain.IsSchemaValidationError(err):
		return schemaValidationConnectError(err)
	case domain.IsValidationError(err):
		return connect.NewError(connect.CodeInvalidArgument, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

func schemaValidationConnectError(err error) *connect.Error {
	connectErr := connect.NewError(connect.CodeInvalidArgument, err)
	var sve *domain.SchemaValidationError
	if errors.As(err, &sve) {
		failure := &configv1.SchemaValidationFailure{}
		for _, v := range sve.Violations {
			failure.Violations = append(failure.Violations, &configv1.SchemaViolation{
				Path:    v.Path,
				Message: v.Message,
				Keyword: v.Keyword,
			})
		}
		if detail, detailErr := connect.NewErrorDetail(failure); detailErr == nil {
			connectErr.AddDetail(detail)
		}
	}

	return connectErr
}
