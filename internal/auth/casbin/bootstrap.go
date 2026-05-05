package casbin

import (
	"context"
	"fmt"
	"slices"

	authpkg "github.com/sergeyslonimsky/elara/internal/auth"
)

// BootstrapEnforcer is the minimal interface required by CheckBootstrapAdmin.
type BootstrapEnforcer interface {
	GetRolesForUser(user, domain string) ([]string, error)
	AddRoleForUser(user, role, domain string) error
}

// CheckBootstrapAdmin checks if email is in adminEmails and has no role:admin assignment yet.
// If both conditions are true, it grants role:admin in domain "*".
// AutoSave on the enforcer's adapter handles persistence automatically.
func CheckBootstrapAdmin(
	ctx context.Context,
	email string,
	adminEmails []string,
	enforcer BootstrapEnforcer,
) error {
	_ = ctx

	if !isAdminEmail(email, adminEmails) {
		return nil
	}

	roles, err := enforcer.GetRolesForUser(email, authpkg.ObjectAll)
	if err != nil {
		return fmt.Errorf("get roles for bootstrap admin: %w", err)
	}

	if slices.Contains(roles, authpkg.RoleAdmin) {
		return nil
	}

	if err = enforcer.AddRoleForUser(email, authpkg.RoleAdmin, authpkg.ObjectAll); err != nil {
		return fmt.Errorf("assign admin role: %w", err)
	}

	return nil
}

func isAdminEmail(email string, adminEmails []string) bool {
	return slices.Contains(adminEmails, email)
}
