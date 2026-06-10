package domain

import "strings"

// SystemGroupSuperAdmin is the canonical name of the bootstrap admins group
// seeded at startup. Membership in this group grants the admin role globally.
//
// Naming convention (EL-50 §3.2): the name is plain regex-safe ("superadmin"),
// and "systemness" is carried by Group.System == true, NOT by a name prefix.
// Casbin sees `group:superadmin` — no double prefix, no regex carve-out.
const SystemGroupSuperAdmin = "superadmin"

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

// Resource-domain prefixes. Every Casbin domain for a scoped object carries a
// type tag so a raw p-rule is self-describing in a debugger ("namespace:prod"
// vs a bare "prod"). DomainAll ("*") is wildcard-as-domain and is NEVER
// prefixed; helpers below short-circuit on it.
const (
	NamespaceResourcePrefix = "namespace:"
	GroupResourcePrefix     = "group:"
)

// GroupResource returns the canonical Casbin domain string for a permission
// scoped to a single group: "group:<name>". Passing DomainAll returns DomainAll
// unchanged so callers can build either a concrete or wildcard grant through
// the same helper without inline branching. name — canonical Group.Name, не UUID.
func GroupResource(name string) string {
	if name == DomainAll {
		return DomainAll
	}

	return GroupResourcePrefix + name
}

// NamespaceResource returns the canonical Casbin domain string for a
// permission scoped to a single namespace: "namespace:<name>". Mirrors
// GroupResource: DomainAll passes through unchanged.
func NamespaceResource(name string) string {
	if name == DomainAll {
		return DomainAll
	}

	return NamespaceResourcePrefix + name
}

// IsGroupSubject reports whether the given Casbin subject string refers to a
// group (i.e. starts with the "group:" prefix). Users appear as bare UUIDs
// so the absence of the prefix is enough to distinguish them.
func IsGroupSubject(subject string) bool {
	return strings.HasPrefix(subject, GroupResourcePrefix)
}

// GroupNameFromSubject strips the "group:" prefix and returns the raw group
// name. If the subject is not a group subject, it is returned unchanged.
func GroupNameFromSubject(subject string) string {
	return strings.TrimPrefix(subject, GroupResourcePrefix)
}

// IsUserSubject reports whether the given Casbin subject refers to a user
// (i.e. is not a group subject). Per EL-50 §5 users are bare UUIDs in Casbin.
func IsUserSubject(subject string) bool {
	return !strings.HasPrefix(subject, GroupResourcePrefix)
}
