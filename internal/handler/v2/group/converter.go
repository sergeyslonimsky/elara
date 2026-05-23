package group

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/permission"
	v1 "github.com/sergeyslonimsky/elara/internal/proto/elara/group/v1"
	groupuc "github.com/sergeyslonimsky/elara/internal/usecase/group"
)

func updateGroupReqToData(req *v1.UpdateGroupRequest) groupuc.UpdateData {
	return groupuc.UpdateData{
		ID:          req.GetId(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Permissions: permission.AssignmentsToDomain(req.GetPermissions()),
		Members:     req.GetMembers(),
		Version:     req.GetVersion(),
	}
}

func domainGroupToProto(g *domain.Group) *v1.Group {
	if g == nil {
		return nil
	}

	return &v1.Group{
		Id:          g.ID,
		Name:        g.Name,
		Members:     g.Members,
		CreatedAt:   timestamppb.New(g.CreatedAt),
		UpdatedAt:   timestamppb.New(g.UpdatedAt),
		IsSystem:    g.System,
		Description: g.Description,
		Version:     g.Version,
		// Permissions are stored in Casbin policy, not on the entity — fetched
		// separately via PDP. Wiring in M5/M6.
	}
}
