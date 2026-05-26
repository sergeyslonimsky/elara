package domain

// SystemGroupSuperAdmin is the canonical name of the bootstrap admins group
// seeded at startup. Membership in this group grants the admin role globally.
const SystemGroupSuperAdmin = "system:superadmin"

// Role constants for RBAC policy assignments.
const (
	RoleAdmin  = "admin"
	RoleWriter = "writer"
	RoleReader = "reader"
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
const (
	ObjectAll       = "*"
	ObjectConfig    = "config"
	ObjectNamespace = "namespace"
	ObjectToken     = "token"
	ObjectClient    = "client"
	ObjectDashboard = "dashboard"
	ObjectUser      = "user"
	ObjectGroup     = "group"
	ObjectPolicy    = "policy"
	ObjectWebhook   = "webhook"
	ObjectSchema    = "schema"
	ObjectTransfer  = "transfer"
)

// Action constants for RBAC permission checks.
const (
	ActionAll    = "*"
	ActionCreate = "create"
	ActionRead   = "read"
	ActionWrite  = "write"
)

// GroupResource returns the canonical Casbin domain string for a permission
// scoped to a single group: "group:<id>". Use this everywhere a permission
// like `Group:Write group:<id>` is constructed — never concatenate the
// "group:" prefix inline.
func GroupResource(id string) string {
	return "group:" + id
}
