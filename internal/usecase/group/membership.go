package group

import (
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
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
	w *authz.PAPTx,
	actor domain.AuthInfo,
	groupName string,
) error {
	perms, err := w.GroupPermissions(groupName)
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
