package domain

type UserStatus string

const (
	UserStatusActive      UserStatus = "active"
	UserStatusDeactivated UserStatus = "deactivated"
)

func (s UserStatus) Valid() bool {
	switch s {
	case UserStatusActive, UserStatusDeactivated:
		return true
	default:
		return false
	}
}
