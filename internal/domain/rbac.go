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
//
// Extension rule: content scoped to a namespace (configs, schemas,
// export/import) is covered by ObjectNamespace — do NOT add a separate object
// for it. Add a new object only for a resource with its own lifecycle or
// access-delegation semantics (e.g. ObjectToken, ObjectWebhook).
type Object string

const (
	ObjectAll       Object = "*"
	ObjectNamespace Object = "namespace"
	ObjectToken     Object = "token"
	ObjectClient    Object = "client"
	ObjectDashboard Object = "dashboard"
	ObjectUser      Object = "user"
	ObjectGroup     Object = "group"
	ObjectPolicy    Object = "policy"
	ObjectWebhook   Object = "webhook"
)

// Action constants for RBAC permission checks.
//
// read/write gate content; write implies read (see ActionGrants). create and
// delete are independent capabilities (e.g. create/delete a namespace).
type Action string

const (
	ActionAll    Action = "*"
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionWrite  Action = "write"
	ActionDelete Action = "delete"
)

// ObjectGrants reports whether a permission granting object `granted` satisfies
// a request for object `required`. A wildcard grant covers any object.
//
// This is the single source of truth for object matching: it backs both the
// Casbin matcher (registered as "objGrants") and the PDP rule scan, so the two
// enforcement paths can never drift.
func ObjectGrants(granted, required Object) bool {
	return granted == ObjectAll || granted == required
}

// ActionGrants reports whether a permission granting action `granted` satisfies
// a request for action `required`. A wildcard grant covers any action, and
// write implies read (you cannot edit what you cannot read).
//
// Single source of truth for action matching: backs both the Casbin matcher
// (registered as "actGrants") and the PDP rule scan.
func ActionGrants(granted, required Action) bool {
	switch {
	case granted == ActionAll || granted == required:
		return true
	case granted == ActionWrite && required == ActionRead:
		return true
	default:
		return false
	}
}

// GroupResource returns the canonical Casbin domain string for a permission
// scoped to a single group: "group:<id>". Use this everywhere a permission
// like `Group:Write group:<id>` is constructed — never concatenate the
// "group:" prefix inline.
func GroupResource(id string) string {
	return "group:" + id
}
