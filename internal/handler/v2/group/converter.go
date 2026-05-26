package group

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v1 "github.com/sergeyslonimsky/elara/internal/proto/elara/group/v1"
)

func domainGroupToProto(g *domain.Group) *v1.Group {
	if g == nil {
		return nil
	}

	return &v1.Group{
		Id:                 g.ID,
		Name:               g.Name,
		Description:        g.Description,
		IsSystem:           g.System,
		CreatedAt:          timestamppb.New(g.CreatedAt),
		UpdatedAt:          timestamppb.New(g.UpdatedAt),
		MetadataVersion:    g.MetadataVersion,
		MembersVersion:     g.MembersVersion,
		PermissionsVersion: g.PermissionsVersion,
	}
}
