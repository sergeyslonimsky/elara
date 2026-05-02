package casbin

import (
	"context"
	"fmt"
	"slices"
)

// BootstrapEnforcer is the minimal interface required by CheckBootstrapAdmin.
type BootstrapEnforcer interface {
	GetRolesForUser(user, domain string) ([]string, error)
	AddRoleForUser(user, role, domain string) error
	SavePolicy(ctx context.Context, loader PolicyLoader) error
}

// CheckBootstrapAdmin checks if email is in adminEmails and has no role:admin assignment yet.
// If both conditions are true, it grants role:admin in domain "*" and saves the policy.
func CheckBootstrapAdmin(
	ctx context.Context,
	email string,
	adminEmails []string,
	enforcer BootstrapEnforcer,
	loader PolicyLoader,
) error {
	if !isAdminEmail(email, adminEmails) {
		return nil
	}

	roles, err := enforcer.GetRolesForUser(email, "*")
	if err != nil {
		return fmt.Errorf("get roles for bootstrap admin: %w", err)
	}

	if slices.Contains(roles, "role:admin") {
		return nil
	}

	if err = enforcer.AddRoleForUser(email, "role:admin", "*"); err != nil {
		return fmt.Errorf("assign admin role: %w", err)
	}

	if err = enforcer.SavePolicy(ctx, loader); err != nil {
		return fmt.Errorf("save policy after bootstrap: %w", err)
	}

	return nil
}

func isAdminEmail(email string, adminEmails []string) bool {
	return slices.Contains(adminEmails, email)
}
