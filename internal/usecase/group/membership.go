package group

import (
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
)

// AuthorizeGrantToUser enforces the anti-escalation invariant for granting a
// single user membership in a group: the actor must hold every permission the
// group currently grants, since adding a member effectively bestows the
// group's full permission set on that user.
//
// Removals narrow a user's permissions and therefore require no escalation
// check.
func AuthorizeGrantToUser(
	pdp *authz.PDP,
	txe *casbin.TxEnforcer,
	actor domain.AuthInfo,
	groupName string,
) error {
	perms, err := loadGroupPermissions(txe, groupName)
	if err != nil {
		return fmt.Errorf("authorize grant: %w", err)
	}

	for _, p := range perms {
		if !pdp.Has(actor.Email, p) {
			return domain.ErrPermissionEscalation
		}
	}

	return nil
}

// loadGroupPermissions reads the p-rules attached to the given group as
// domain.Permission values. Lives here because AuthorizeGrantToUser still
// works directly against a casbin.TxEnforcer; the analogous read in the
// Update path has migrated to authz.PAPTx.GroupPermissions.
func loadGroupPermissions(txe *casbin.TxEnforcer, name string) ([]domain.Permission, error) {
	rules, err := txe.GetPermissionsForSubject(casbin.GroupSubject(name))
	if err != nil {
		return nil, fmt.Errorf("get permissions: %w", err)
	}

	out := make([]domain.Permission, 0, len(rules))
	for _, r := range rules {
		out = append(out, domain.Permission{Object: r[2], Action: r[3], Domain: r[1]})
	}

	return out, nil
}
