package internal

import (
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// GroupMeta is the bbolt JSON shape for a group entity.
//
// Membership (user→group) and permissions live exclusively in Casbin
// (g-rules / p-rules); this struct only carries the entity metadata
// bbolt is authoritative for. The three version counters track
// optimistic-lock state independently per editable slot.
type GroupMeta struct {
	Name               string    `json:"name"`
	DisplayName        string    `json:"display_name,omitempty"`
	Description        string    `json:"description,omitempty"`
	System             bool      `json:"system,omitempty"`
	MetadataVersion    int64     `json:"metadata_version,omitempty"`
	MembersVersion     int64     `json:"members_version,omitempty"`
	PermissionsVersion int64     `json:"permissions_version,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func DomainToGroupMeta(g *domain.Group) GroupMeta {
	return GroupMeta{
		Name:               g.Name,
		DisplayName:        g.DisplayName,
		Description:        g.Description,
		System:             g.System,
		MetadataVersion:    g.MetadataVersion,
		MembersVersion:     g.MembersVersion,
		PermissionsVersion: g.PermissionsVersion,
		CreatedAt:          g.CreatedAt,
		UpdatedAt:          g.UpdatedAt,
	}
}

func GroupMetaToDomain(m GroupMeta) *domain.Group {
	return &domain.Group{
		Name:               m.Name,
		DisplayName:        m.DisplayName,
		Description:        m.Description,
		System:             m.System,
		MetadataVersion:    m.MetadataVersion,
		MembersVersion:     m.MembersVersion,
		PermissionsVersion: m.PermissionsVersion,
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
	}
}
