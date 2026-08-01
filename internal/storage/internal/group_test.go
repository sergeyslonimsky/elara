package internal_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
	storageinternal "github.com/sergeyslonimsky/elara/internal/storage/internal"
)

func TestDomainToGroupMeta(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name  string
		group *domain.Group
		want  storageinternal.GroupMeta
	}{
		{
			name: "full group",
			group: &domain.Group{
				Name:               "group-1",
				DisplayName:        "Group One",
				Description:        "desc",
				System:             true,
				MetadataVersion:    1,
				MembersVersion:     2,
				PermissionsVersion: 3,
				CreatedAt:          now,
				UpdatedAt:          now,
			},
			want: storageinternal.GroupMeta{
				Name:               "group-1",
				DisplayName:        "Group One",
				Description:        "desc",
				System:             true,
				MetadataVersion:    1,
				MembersVersion:     2,
				PermissionsVersion: 3,
				CreatedAt:          now,
				UpdatedAt:          now,
			},
		},
		{
			name:  "zero value group",
			group: &domain.Group{},
			want:  storageinternal.GroupMeta{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.DomainToGroupMeta(tt.group)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGroupMetaToDomain(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name string
		meta storageinternal.GroupMeta
		want *domain.Group
	}{
		{
			name: "full meta",
			meta: storageinternal.GroupMeta{
				Name:               "group-1",
				DisplayName:        "Group One",
				Description:        "desc",
				System:             true,
				MetadataVersion:    1,
				MembersVersion:     2,
				PermissionsVersion: 3,
				CreatedAt:          now,
				UpdatedAt:          now,
			},
			want: &domain.Group{
				Name:               "group-1",
				DisplayName:        "Group One",
				Description:        "desc",
				System:             true,
				MetadataVersion:    1,
				MembersVersion:     2,
				PermissionsVersion: 3,
				CreatedAt:          now,
				UpdatedAt:          now,
			},
		},
		{
			name: "zero value meta",
			meta: storageinternal.GroupMeta{},
			want: &domain.Group{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.GroupMetaToDomain(tt.meta)
			assert.Equal(t, tt.want, got)
		})
	}
}
