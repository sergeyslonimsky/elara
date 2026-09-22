package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
)

func TestProtoSortToDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sort *commonv1.SortRequest
		want domain.SortParams
	}{
		{
			name: "nil sort returns zero value",
			sort: nil,
			want: domain.SortParams{},
		},
		{
			name: "ascending direction",
			sort: &commonv1.SortRequest{
				Field:     "name",
				Direction: commonv1.SortDirection_SORT_DIRECTION_ASC,
			},
			want: domain.SortParams{Field: "name", Desc: false},
		},
		{
			name: "descending direction",
			sort: &commonv1.SortRequest{
				Field:     "name",
				Direction: commonv1.SortDirection_SORT_DIRECTION_DESC,
			},
			want: domain.SortParams{Field: "name", Desc: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, ProtoSortToDomain(tc.sort))
		})
	}
}
