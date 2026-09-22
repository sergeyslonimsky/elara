package v2

import (
	"github.com/sergeyslonimsky/elara/internal/domain"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
)

// ProtoSortToDomain converts a client-supplied SortRequest into domain sort
// params. A nil request (sort unspecified) maps to the zero value.
func ProtoSortToDomain(s *commonv1.SortRequest) domain.SortParams {
	if s == nil {
		return domain.SortParams{}
	}

	return domain.SortParams{
		Field: s.GetField(),
		Desc:  s.GetDirection() == commonv1.SortDirection_SORT_DIRECTION_DESC,
	}
}
