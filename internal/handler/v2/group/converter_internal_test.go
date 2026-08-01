package group

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v1 "github.com/sergeyslonimsky/elara/internal/proto/elara/group/v1"
)

func TestDomainGroupToProto(t *testing.T) {
	t.Parallel()

	created := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		g    *domain.Group
		want *v1.Group
	}{
		{
			name: "nil group returns nil",
			g:    nil,
			want: nil,
		},
		{
			name: "maps all fields including system flag and versions",
			g: &domain.Group{
				Name:               "engineers",
				DisplayName:        "Engineers",
				Description:        "Engineering team",
				System:             true,
				MetadataVersion:    3,
				MembersVersion:     7,
				PermissionsVersion: 2,
				CreatedAt:          created,
				UpdatedAt:          updated,
			},
			want: &v1.Group{
				Name:               "engineers",
				DisplayName:        "Engineers",
				Description:        "Engineering team",
				IsSystem:           true,
				MetadataVersion:    3,
				MembersVersion:     7,
				PermissionsVersion: 2,
				CreatedAt:          timestamppb.New(created),
				UpdatedAt:          timestamppb.New(updated),
			},
		},
		{
			name: "non-system group maps IsSystem false",
			g: &domain.Group{
				Name:      "regular",
				System:    false,
				CreatedAt: created,
				UpdatedAt: updated,
			},
			want: &v1.Group{
				Name:      "regular",
				IsSystem:  false,
				CreatedAt: timestamppb.New(created),
				UpdatedAt: timestamppb.New(updated),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := domainGroupToProto(tc.g)
			assert.Equal(t, tc.want, got)
		})
	}
}
