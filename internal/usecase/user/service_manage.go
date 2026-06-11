package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

// CreateData carries the parameters accepted by Create.
//
// InitialGroupIDs are applied atomically with the user record. The handler
// gate already covers the coarse permission; this usecase enforces the
// per-group authz + anti-escalation invariants the same way UpdateGroups
// does for an existing user.
type CreateData struct {
	Email           string
	DisplayName     string
	InitialPassword string // required basic-auth; must be empty in OIDC (handler-enforced)
	InitialGroupIDs []string
}

// CreateResult bundles the user record with the canonical membership state
// just-after-creation so the handler can render the response.
type CreateResult struct {
	User              *domain.User
	GroupIDs          []string
	MembershipVersion int64
}

// Create creates a new user.
//
// Authorization (per proto contract):
//   - User:Create * (global), OR
//   - InitialGroupIDs is non-empty AND actor holds Group:Write on each id.
//
// The fast-fail pre-check before opening the tx (s.preauthorize) covers
// both clauses; the per-id Group:Write loop runs again inside the tx
// alongside RequireMembershipGrant so concurrent revocation between
// pre-check and apply cannot bypass the gate.
func (s *Service) Create(
	ctx context.Context,
	actor domain.AuthInfo,
	data CreateData,
) (*CreateResult, error) {
	user, err := newUserFromCreateData(data)
	if err != nil {
		return nil, err
	}

	if err := s.preauthorize(actor, data.InitialGroupIDs); err != nil {
		return nil, err
	}

	var result *CreateResult

	err = s.pap.Write(ctx, func(ctx context.Context, w *authz.PAPTx) error {
		if err := s.persistUser(ctx, user, data.InitialPassword); err != nil {
			return err
		}
		groupIDs, version, err := s.applyInitialGroups(
			ctx,
			w,
			actor,
			user,
			data.InitialGroupIDs,
		)
		if err != nil {
			return err
		}
		user.MembershipVersion = version
		result = &CreateResult{User: user, GroupIDs: groupIDs, MembershipVersion: version}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create user transaction: %w", err)
	}

	return result, nil
}

// newUserFromCreateData builds a validated User entity from request data.
//
// Identity model (per EL-50 §3.3.1 pre-provisioning flow):
//   - basic auth: identity is created immediately with the normalized email
//     as Subject — admin knows the "login" at create time.
//   - OIDC: Identities is left EMPTY. The OIDC sub is unknown until the user
//     actually logs in for the first time; the first OIDC-callback then
//     links {oidc:<issuer>, sub} via email-fallback (resolver.ResolveOIDC).
func newUserFromCreateData(data CreateData) (*domain.User, error) {
	normalizedEmail, err := domain.NormalizeEmail(data.Email)
	if err != nil {
		return nil, fmt.Errorf("normalize email: %w", err)
	}

	user := &domain.User{
		ID:          uuid.New(),
		Email:       normalizedEmail,
		DisplayName: data.DisplayName,
		Status:      domain.UserStatusActive,
	}

	if data.InitialPassword == "" {
		// OIDC pre-provision: no identity until first login.
		user.Identities = nil
	} else {
		identity, err := domain.NewIdentity(domain.ProviderBasic, normalizedEmail)
		if err != nil {
			return nil, fmt.Errorf("construct basic identity: %w", err)
		}
		user.Identities = []domain.Identity{identity}
	}

	if err := user.Validate(); err != nil {
		return nil, fmt.Errorf("validate user: %w", err)
	}

	return user, nil
}

// preauthorize is the fast-fail authorization check that runs before any
// transaction is opened. It implements the proto's "User:Create * OR
// initial_group_ids non-empty + Group:Write on each id" gate.
//
// The actual enforcement happens inside the tx (authorizeInitialGroups +
// RequireMembershipGrant) so concurrent revocation between this pre-check
// and the apply is caught — preauthorize is purely an optimization.
func (s *Service) preauthorize(actor domain.AuthInfo, ids []string) error {
	if s.pdp.HasGlobal(actor.UserID, domain.ObjectUser, domain.ActionCreate) {
		return nil
	}
	if len(ids) == 0 {
		return domain.ErrForbidden
	}

	return s.authorizeInitialGroups(actor, ids)
}

// authorizeInitialGroups checks Group:Write per id. Called both pre-tx
// (via preauthorize) and inside the tx (via applyInitialGroups) — the
// duplicate is intentional: pre-check avoids opening a tx when the caller
// clearly has no scope, in-tx check closes the TOCTOU window.
func (s *Service) authorizeInitialGroups(actor domain.AuthInfo, ids []string) error {
	for _, id := range ids {
		if !s.pdp.HasGroup(actor.UserID, id, domain.ActionWrite) {
			return domain.ErrForbidden
		}
	}

	return nil
}

// persistUser writes the user record and (in basic-auth mode) the initial
// password hash, marking password_change_required on first login.
func (s *Service) persistUser(
	ctx context.Context,
	user *domain.User,
	initialPassword string,
) error {
	if err := s.users.Create(ctx, user); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	if initialPassword == "" {
		return nil
	}
	hash, err := auth.HashPassword(initialPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := s.store.SetPassword(ctx, user.ID, hash, true); err != nil {
		return fmt.Errorf("set password: %w", err)
	}

	return nil
}

// applyInitialGroups adds the new user to each initial group with full
// anti-escalation. Returns the final group-id slice (echoed back to the
// caller). MembershipVersion is always initialised to 1: proto3 JSON omits
// int64(0), which would make optimistic concurrency impossible on the very
// first UpdateUserGroups call for users created without initial groups.
func (s *Service) applyInitialGroups(
	ctx context.Context,
	papTx *authz.PAPTx,
	actor domain.AuthInfo,
	user *domain.User,
	ids []string,
) ([]string, int64, error) {
	const initialVersion = int64(1)
	if err := s.store.SetMembershipVersion(ctx, user.ID, initialVersion); err != nil {
		return nil, 0, fmt.Errorf("persist initial membership version: %w", err)
	}

	if len(ids) == 0 {
		return []string{}, initialVersion, nil
	}

	// Re-check Group:Write inside the tx: preauthorize ran outside and a
	// concurrent admin could have revoked the actor's scope between then
	// and now. Mirrors the in-tx pattern used by DeleteUser/ResetPassword.
	if err := s.authorizeInitialGroups(actor, ids); err != nil {
		return nil, 0, err
	}

	desired, err := loadGroupsByNames(ctx, s.groups, ids)
	if err != nil {
		return nil, 0, err
	}

	for _, id := range ids {
		if err := s.scope.RequireMembershipGrant(actor.UserID, desired[id].Name); err != nil {
			return nil, 0, fmt.Errorf("grant group %s: %w", id, err)
		}
	}

	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = append(names, desired[id].Name)
	}
	if err := papTx.ApplyUserMembershipDeltas(user.ID.String(), names, nil); err != nil {
		return nil, 0, fmt.Errorf("pap apply user memberships: %w", err)
	}

	return append([]string(nil), ids...), initialVersion, nil
}

// Delete removes the user along with all their memberships.
//
// Authorization: User:Write * (global) OR derived (target ∈ any group with
// Group:Write). Plus anti-escalation: caller must hold every permission
// target currently has (impersonation cannot escalate privilege).
//
// Self-delete is rejected. The last-admin guard prevents deleting the only
// remaining holder of the wildcard admin grant.
//
// All checks run inside PAP.Write so they observe the same snapshot as the
// apply step: no TOCTOU window between authorize and delete.
func (s *Service) Delete(ctx context.Context, actor domain.AuthInfo, userID uuid.UUID) error {
	err := s.pap.Write(ctx, func(ctx context.Context, papTx *authz.PAPTx) error {
		user, err := s.store.GetByID(ctx, userID)
		if err != nil {
			if errors.Is(err, storage.ErrResourceNotFound) {
				return fmt.Errorf("get user: %w", domain.ErrNotFound)
			}

			return fmt.Errorf("get user: %w", err)
		}
		if actor.UserID == user.ID.String() {
			return domain.NewValidationError("user_id", "cannot delete your own account")
		}
		if err := s.authorizeUserWrite(ctx, actor, user.ID.String()); err != nil {
			return err
		}
		if err := s.validateLastAdmin(user.ID.String()); err != nil {
			return err
		}
		if err := s.store.Delete(ctx, user.ID); err != nil {
			return fmt.Errorf("delete user: %w", err)
		}
		if err := papTx.DeleteUser(user.ID.String()); err != nil {
			return fmt.Errorf("pap delete user: %w", err)
		}

		if err := s.sessions.RevokeAllForUser(ctx, user.ID.String(), actor.UserID, "user deleted"); err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("delete user transaction: %w", err)
	}

	return nil
}

// GetResult bundles a user with the group IDs visible to the caller and
// their current membership version. Memberships outside the caller's
// Group:Read scope are filtered out.
type GetResult struct {
	User              *domain.User
	VisibleGroupIDs   []string
	MembershipVersion int64
}

// Get returns the target user along with the subset of their memberships
// the caller is permitted to see.
//
// Authorization: User:Read * OR target ∈ any group caller can read. When
// the caller has neither, the response is NotFound (not Forbidden), uniform
// with the missing-user case, so the endpoint cannot be used to enumerate
// users by email.
func (s *Service) Get(
	ctx context.Context,
	actor domain.AuthInfo,
	userID uuid.UUID,
) (*GetResult, error) {
	user, err := s.store.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrResourceNotFound) {
			return nil, fmt.Errorf("get user: %w", domain.ErrNotFound)
		}

		return nil, fmt.Errorf("get user: %w", err)
	}

	if !s.scope.CanReadUser(ctx, actor.UserID, user.ID.String()) {
		return nil, fmt.Errorf("get user: %w", domain.NewNotFoundError("user", userID.String()))
	}

	visible, err := s.scope.VisibleUserGroupNames(ctx, actor.UserID, user.ID.String())
	if err != nil {
		return nil, fmt.Errorf("visible user group ids: %w", err)
	}

	return &GetResult{
		User:              user,
		VisibleGroupIDs:   visible,
		MembershipVersion: user.MembershipVersion,
	}, nil
}

// validateLastAdmin prevents removing the final member of the superadmin
// group, which would lock everyone out of administration. Admin privilege is
// held only via membership in domain.SystemGroupSuperAdmin (groups-only RBAC).
func (s *Service) validateLastAdmin(targetID string) error {
	members := s.pap.GroupMembers(domain.SystemGroupSuperAdmin)
	if len(members) == 1 && members[0] == targetID {
		return domain.NewValidationError("id", "cannot delete the last admin")
	}

	return nil
}
