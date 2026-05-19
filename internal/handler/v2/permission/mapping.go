// Package permission converts between proto PermissionAssignment enums and
// domain string constants. Handler layer concern — domain MUST NOT import
// commonv1; handlers MUST NOT re-implement these mappings.
package permission

import (
	"github.com/sergeyslonimsky/elara/internal/domain"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
)

// ObjectToProto maps a domain object string to the proto enum.
// Unknown values yield PERMISSION_OBJECT_UNSPECIFIED.
func ObjectToProto(s string) commonv1.PermissionObject {
	switch s {
	case domain.ObjectNamespace:
		return commonv1.PermissionObject_PERMISSION_OBJECT_NAMESPACE
	case domain.ObjectConfig:
		return commonv1.PermissionObject_PERMISSION_OBJECT_CONFIG
	case domain.ObjectUser:
		return commonv1.PermissionObject_PERMISSION_OBJECT_USER
	case domain.ObjectGroup:
		return commonv1.PermissionObject_PERMISSION_OBJECT_GROUP
	case domain.ObjectToken:
		return commonv1.PermissionObject_PERMISSION_OBJECT_TOKEN
	case domain.ObjectWebhook:
		return commonv1.PermissionObject_PERMISSION_OBJECT_WEBHOOK
	case domain.ObjectAll:
		return commonv1.PermissionObject_PERMISSION_OBJECT_ALL
	default:
		return commonv1.PermissionObject_PERMISSION_OBJECT_UNSPECIFIED
	}
}

// ObjectToDomain maps a proto object enum to its domain string constant.
// Returns "" only for PERMISSION_OBJECT_UNSPECIFIED (and unknown enum ints);
// PERMISSION_OBJECT_ALL maps to domain.ObjectAll.
func ObjectToDomain(o commonv1.PermissionObject) string {
	switch o {
	case commonv1.PermissionObject_PERMISSION_OBJECT_NAMESPACE:
		return domain.ObjectNamespace
	case commonv1.PermissionObject_PERMISSION_OBJECT_CONFIG:
		return domain.ObjectConfig
	case commonv1.PermissionObject_PERMISSION_OBJECT_USER:
		return domain.ObjectUser
	case commonv1.PermissionObject_PERMISSION_OBJECT_GROUP:
		return domain.ObjectGroup
	case commonv1.PermissionObject_PERMISSION_OBJECT_TOKEN:
		return domain.ObjectToken
	case commonv1.PermissionObject_PERMISSION_OBJECT_WEBHOOK:
		return domain.ObjectWebhook
	case commonv1.PermissionObject_PERMISSION_OBJECT_ALL:
		return domain.ObjectAll
	case commonv1.PermissionObject_PERMISSION_OBJECT_UNSPECIFIED:
		return ""
	default:
		return ""
	}
}

// ActionToProto maps a domain action string to the proto enum.
// Unknown values yield PERMISSION_ACTION_UNSPECIFIED.
func ActionToProto(s string) commonv1.PermissionAction {
	switch s {
	case domain.ActionRead:
		return commonv1.PermissionAction_PERMISSION_ACTION_READ
	case domain.ActionWrite:
		return commonv1.PermissionAction_PERMISSION_ACTION_WRITE
	case domain.ActionAll:
		return commonv1.PermissionAction_PERMISSION_ACTION_ALL
	default:
		return commonv1.PermissionAction_PERMISSION_ACTION_UNSPECIFIED
	}
}

// ActionToDomain maps a proto action enum to its domain string constant.
// Returns "" only for PERMISSION_ACTION_UNSPECIFIED (and unknown enum ints);
// PERMISSION_ACTION_ALL maps to domain.ActionAll.
func ActionToDomain(a commonv1.PermissionAction) string {
	switch a {
	case commonv1.PermissionAction_PERMISSION_ACTION_READ:
		return domain.ActionRead
	case commonv1.PermissionAction_PERMISSION_ACTION_WRITE:
		return domain.ActionWrite
	case commonv1.PermissionAction_PERMISSION_ACTION_ALL:
		return domain.ActionAll
	case commonv1.PermissionAction_PERMISSION_ACTION_UNSPECIFIED:
		return ""
	default:
		return ""
	}
}

// AssignmentToProto converts a domain.Permission to its proto representation.
func AssignmentToProto(p domain.Permission) *commonv1.PermissionAssignment {
	return &commonv1.PermissionAssignment{
		Object: ObjectToProto(p.Object),
		Action: ActionToProto(p.Action),
		Domain: p.Domain,
	}
}

// AssignmentToDomain converts a proto PermissionAssignment to a domain.Permission.
// The second return value is false only when either the object or action is
// UNSPECIFIED (i.e. invalid input). PERMISSION_OBJECT_ALL / PERMISSION_ACTION_ALL
// are valid and map to domain.ObjectAll / domain.ActionAll.
func AssignmentToDomain(in *commonv1.PermissionAssignment) (domain.Permission, bool) {
	if in == nil {
		return domain.Permission{}, false
	}

	obj := ObjectToDomain(in.GetObject())
	act := ActionToDomain(in.GetAction())
	if obj == "" || act == "" {
		return domain.Permission{}, false
	}

	return domain.Permission{
		Object: obj,
		Action: act,
		Domain: in.GetDomain(),
	}, true
}

// AssignmentsToDomain converts a slice of proto assignments, silently dropping
// invalid (UNSPECIFIED) entries. Returns nil for nil/empty input.
func AssignmentsToDomain(in []*commonv1.PermissionAssignment) []domain.Permission {
	if len(in) == 0 {
		return nil
	}

	out := make([]domain.Permission, 0, len(in))
	for _, p := range in {
		if perm, ok := AssignmentToDomain(p); ok {
			out = append(out, perm)
		}
	}

	return out
}

// AssignmentsToProto converts a slice of domain.Permission to proto assignments.
// Returns nil for nil/empty input.
func AssignmentsToProto(in []domain.Permission) []*commonv1.PermissionAssignment {
	if len(in) == 0 {
		return nil
	}

	out := make([]*commonv1.PermissionAssignment, 0, len(in))
	for _, p := range in {
		out = append(out, AssignmentToProto(p))
	}

	return out
}
