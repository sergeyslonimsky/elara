package domain

// SystemGroupSuperAdmin is the canonical name of the bootstrap admins group
// seeded at startup. Membership in this group grants the admin role globally.
const SystemGroupSuperAdmin = "system:superadmin"

// Role constants for RBAC policy assignments.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleWriter Role = "writer"
	RoleReader Role = "reader"
)

// Domain constants. The Casbin domain field (third column of a g-rule, second
// column of an Enforce request) names a namespace; the wildcards below carry
// different intent at different call sites — keep them distinguishable.
const (
	// DomainAll matches any namespace. Used as r.dom when checking a global
	// permission and as p.dom on p-rules that grant a role in every namespace.
	DomainAll = "*"

	// MembershipDomain is the domain stored on group-membership g-rules
	// (g, <user>, group:<name>, *). It expresses "membership is global, not
	// tied to a specific namespace" — the user belongs to the group; what
	// the group is allowed to do in which domain is encoded by separate
	// group->role g-rules.
	MembershipDomain = "*"
)

// Object constants for RBAC resource identification.
type Object string

const (
	ObjectAll       Object = "*"
	ObjectConfig    Object = "config"
	ObjectNamespace Object = "namespace"
	ObjectToken     Object = "token"
	ObjectClient    Object = "client"
	ObjectDashboard Object = "dashboard"
	ObjectUser      Object = "user"
	ObjectGroup     Object = "group"
	ObjectPolicy    Object = "policy"
	ObjectWebhook   Object = "webhook"
	ObjectSchema    Object = "schema"
	ObjectTransfer  Object = "transfer"
)

// Action constants for RBAC permission checks.
type Action string

const (
	ActionAll    Action = "*"
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionWrite  Action = "write"
)

// GroupResource returns the canonical Casbin domain string for a permission
// scoped to a single group: "group:<id>". Use this everywhere a permission
// like `Group:Write group:<id>` is constructed — never concatenate the
// "group:" prefix inline.
func GroupResource(id string) string {
	return "group:" + id
}
