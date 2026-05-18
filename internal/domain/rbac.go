package domain

import "strings"

// GroupSubjectPrefix is prepended to group names when they appear as a Casbin subject.
// This keeps the group namespace disjoint from user emails so a group named "alice@x"
// can never collide with the user "alice@x".
const GroupSubjectPrefix = "group:"

// SystemGroupSuperAdmin is the canonical name of the bootstrap admins group seeded
// at startup. Membership in this group grants the admin role globally.
const SystemGroupSuperAdmin = "system:superadmin"

// GroupSubject returns the Casbin subject string for the given group name.
func GroupSubject(name string) string {
	return GroupSubjectPrefix + name
}

// IsGroupSubject reports whether the given Casbin subject refers to a group.
func IsGroupSubject(subject string) bool {
	return strings.HasPrefix(subject, GroupSubjectPrefix)
}

// GroupNameFromSubject strips the group prefix and returns the raw group name.
// If the subject is not a group subject, it is returned unchanged.
func GroupNameFromSubject(subject string) string {
	return strings.TrimPrefix(subject, GroupSubjectPrefix)
}

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
	ActionAll   = "*"
	ActionRead  = "read"
	ActionWrite = "write"
)
