package casbin

import "strings"

// GroupSubjectPrefix is prepended to group names when they appear as a
// Casbin subject. This keeps the group namespace disjoint from user emails
// so a group named "alice@x" can never collide with the user "alice@x".
const GroupSubjectPrefix = "group:"

// GroupSubject returns the Casbin subject string for the given group name.
func GroupSubject(name string) string {
	return GroupSubjectPrefix + name
}

// IsGroupSubject reports whether the given Casbin subject refers to a group.
func IsGroupSubject(subject string) bool {
	return strings.HasPrefix(subject, GroupSubjectPrefix)
}

// GroupNameFromSubject strips the group prefix and returns the raw group
// name. If the subject is not a group subject, it is returned unchanged.
func GroupNameFromSubject(subject string) string {
	return strings.TrimPrefix(subject, GroupSubjectPrefix)
}
