package group

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/usecase/filter"
	"github.com/sergeyslonimsky/elara/internal/util/sliceutil"
)

// UpdateData carries the metadata-only fields the group entity owns in
// bbolt. Members and permissions are managed by UpdateMembers and
// UpdatePermissions respectively.
type UpdateData struct {
	Name                    string
	DisplayName             string
	Description             string
	ExpectedMetadataVersion *int64
}

// Update mutates a group's metadata (display name, description). MetadataVersion
// is bumped. RENAMING is not supported in this version - Name is immutable.
func (s *Service) Update(
	ctx context.Context,
	_ domain.AuthInfo,
	data UpdateData,
) (*domain.Group, error) {
	var updated *domain.Group

	err := s.pap.Write(ctx, func(ctx context.Context, w *authz.PAPTx) error {
		existing, err := s.loadMutableGroup(ctx, data.Name)
		if err != nil {
			return err
		}
		if err := domain.CheckVersion(data.ExpectedMetadataVersion, existing.MetadataVersion); err != nil {
			return fmt.Errorf("check version: %w", err)
		}

		existing.DisplayName = data.DisplayName
		existing.Description = data.Description
		existing.MetadataVersion++
		existing.UpdatedAt = time.Now().UTC()

		if err := s.store.Update(ctx, existing); err != nil {
			return fmt.Errorf(errUpdateGroup, err)
		}
		updated = existing

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update group: %w", err)
	}

	return updated, nil
}

// UpdateMembersData carries the explicit add/remove delta for membership.
type UpdateMembersData struct {
	GroupName              string
	AddEmails              []string
	RemoveEmails           []string
	ExpectedMembersVersion *int64
}

// UpdateMembersResult bundles the updated group with the member list
// visible to the caller (filtered per derived User:Read).
type UpdateMembersResult struct {
	Group          *domain.Group
	VisibleMembers []string
}

// UpdateMembers applies an explicit membership delta. Add of an existing
// member is a no-op; remove of an absent member is a no-op. An email
// appearing in both add_emails and remove_emails returns InvalidArgument.
//
// Anti-escalation: each effective added email receives the group's full
// permission set, so the actor must hold every one of those permissions.
// Removals narrow and skip the escalation check.
func (s *Service) UpdateMembers(
	ctx context.Context,
	actor domain.AuthInfo,
	data UpdateMembersData,
) (*UpdateMembersResult, error) {
	if v, dup := sliceutil.FirstOverlap(data.AddEmails, data.RemoveEmails); dup {
		return nil, domain.NewValidationError(
			"email",
			fmt.Sprintf("%q appears in both add and remove", v),
		)
	}

	var result *UpdateMembersResult

	err := s.pap.Write(ctx, func(ctx context.Context, w *authz.PAPTx) error {
		existing, err := s.loadMutableGroup(ctx, data.GroupName)
		if err != nil {
			return err
		}
		if err := domain.CheckVersion(data.ExpectedMembersVersion, existing.MembersVersion); err != nil {
			return fmt.Errorf("check version: %w", err)
		}

		current := s.pap.GroupMembers(existing.Name)
		currentSet := sliceutil.ToSet(current)
		added := sliceutil.NotIn(data.AddEmails, currentSet)
		removed := sliceutil.In(data.RemoveEmails, currentSet)

		if len(added) > 0 {
			if err := s.scope.RequireMembershipGrant(actor.UserID, existing.Name); err != nil {
				return fmt.Errorf("require membership grant: %w", err)
			}
		}

		if len(added) == 0 && len(removed) == 0 {
			result = &UpdateMembersResult{
				Group:          existing,
				VisibleMembers: s.filterVisibleMembers(ctx, actor, current),
			}

			return nil
		}

		if err := s.commitMemberDelta(ctx, w, existing, added, removed); err != nil {
			return err
		}

		post := sliceutil.ComposePost(current, added, removed)
		result = &UpdateMembersResult{
			Group:          existing,
			VisibleMembers: s.filterVisibleMembers(ctx, actor, post),
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update group members: %w", err)
	}

	return result, nil
}

// UpdatePermissionsData carries the explicit add/remove delta for the
// group's permission set.
type UpdatePermissionsData struct {
	GroupName                  string
	Add                        []domain.Permission
	Remove                     []domain.Permission
	ExpectedPermissionsVersion *int64
}

// UpdatePermissionsResult bundles the updated group with its current
// permission set.
type UpdatePermissionsResult struct {
	Group       *domain.Group
	Permissions []domain.Permission
}

// UpdatePermissions applies an explicit permission delta. Add of an
// existing permission is a no-op; remove of an absent permission is a
// no-op. A permission in both add and remove returns InvalidArgument.
//
// Anti-escalation:
//   - per-delta: actor must hold each effectively-added permission.
//   - cascade: if the group has members and any delta applies, actor
//     must hold every permission the group will hold post-update —
//     because the change cascades to existing members through their
//     membership g-rules.
func (s *Service) UpdatePermissions(
	ctx context.Context,
	actor domain.AuthInfo,
	data UpdatePermissionsData,
) (*UpdatePermissionsResult, error) {
	if err := validatePermissionDeltas(data.Add, data.Remove); err != nil {
		return nil, err
	}

	var result *UpdatePermissionsResult

	err := s.pap.Write(ctx, func(ctx context.Context, w *authz.PAPTx) error {
		existing, err := s.loadMutableGroup(ctx, data.GroupName)
		if err != nil {
			return err
		}
		if err := domain.CheckVersion(data.ExpectedPermissionsVersion, existing.PermissionsVersion); err != nil {
			return fmt.Errorf("check version: %w", err)
		}

		current, err := w.GroupPermissions(existing.Name)
		if err != nil {
			return fmt.Errorf("pap group permissions: %w", err)
		}
		currentSet := sliceutil.ToSet(current)
		added := sliceutil.NotIn(data.Add, currentSet)
		removed := sliceutil.In(data.Remove, currentSet)

		if err := s.boundaryCheckPerms(actor, added); err != nil {
			return err
		}

		if len(added)+len(removed) == 0 {
			result = &UpdatePermissionsResult{Group: existing, Permissions: current}

			return nil
		}

		post := sliceutil.ComposePost(current, added, removed)
		if err := s.cascadeCheckPerms(actor, existing.Name, post); err != nil {
			return err
		}

		if err := s.commitPermissionDelta(ctx, w, existing, added, removed); err != nil {
			return err
		}

		result = &UpdatePermissionsResult{Group: existing, Permissions: post}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update group permissions: %w", err)
	}

	return result, nil
}

// commitMemberDelta applies the membership delta in the PAP and persists
// the bumped MembersVersion. Extracted from UpdateMembers to keep that
// function under the cyclop threshold; the steps must remain in this order
// because the store row carries MembersVersion as the optimistic-lock seed
// for subsequent calls.
func (s *Service) commitMemberDelta(
	ctx context.Context,
	w *authz.PAPTx,
	existing *domain.Group,
	added, removed []string,
) error {
	if err := w.ApplyMemberDeltas(existing.Name, added, removed); err != nil {
		return fmt.Errorf("apply member deltas: %w", err)
	}
	existing.MembersVersion++
	existing.UpdatedAt = time.Now().UTC()
	if err := s.store.Update(ctx, existing); err != nil {
		return fmt.Errorf(errUpdateGroup, err)
	}

	return nil
}

// commitPermissionDelta applies the permission delta in the PAP and
// persists the bumped PermissionsVersion. Mirrors commitMemberDelta; lives
// here so UpdatePermissions stays under the cyclop threshold.
func (s *Service) commitPermissionDelta(
	ctx context.Context,
	w *authz.PAPTx,
	existing *domain.Group,
	added, removed []domain.Permission,
) error {
	if err := w.ApplyPermissionDeltas(existing.Name, added, removed); err != nil {
		return fmt.Errorf("apply permission deltas: %w", err)
	}
	existing.PermissionsVersion++
	existing.UpdatedAt = time.Now().UTC()
	if err := s.store.Update(ctx, existing); err != nil {
		return fmt.Errorf(errUpdateGroup, err)
	}

	return nil
}

// boundaryCheckPerms asserts each effectively-added permission lies within
// the actor's own permission set.
func (s *Service) boundaryCheckPerms(actor domain.AuthInfo, added []domain.Permission) error {
	for _, p := range added {
		if !s.pdp.Has(actor.UserID, p) {
			return domain.ErrPermissionEscalation
		}
	}

	return nil
}

// validatePermissionDeltas runs all pre-tx input checks on an UpdatePermissions
// delta: the add/remove sets must not intersect, and every added assignment
// must be catalog-valid. Extracted from UpdatePermissions to keep that method
// under the cyclop threshold.
func validatePermissionDeltas(adds, removes []domain.Permission) error {
	if p, dup := sliceutil.FirstOverlap(adds, removes); dup {
		return domain.NewValidationError(
			"permissions",
			fmt.Sprintf("%s:%s on %s appears in both add and remove", p.Object, p.Action, p.Domain),
		)
	}
	for _, p := range adds {
		if err := validatePermissionAssignment(p); err != nil {
			return err
		}
	}

	return nil
}

// validatePermissionAssignment rejects assignments that are syntactically
// well-formed but semantically meaningless against the catalog
// (filter.LookupCatalogEntry). Guards the API from clients bypassing the UI.
//
// Checks are ordered semantically — existence → action allowed → domain shape
// — so the error surfaced to the operator names the deepest legitimate problem
// without leaking redundant follow-ups. domain.NewValidationError maps to
// InvalidArgument at the handler boundary.
func validatePermissionAssignment(p domain.Permission) error {
	entry, ok := filter.LookupCatalogEntry(p.Object)
	if !ok {
		return domain.NewValidationError(
			"permissions",
			fmt.Sprintf("object %q is not assignable", p.Object),
		)
	}
	if err := validateAssignmentAction(entry, p.Action); err != nil {
		return err
	}

	return validateAssignmentDomain(entry.Scope, p.Object, p.Domain)
}

// validateAssignmentAction enforces that the action is one of the actions
// the catalog lists as meaningful for entry.Object.
func validateAssignmentAction(entry filter.CatalogEntry, action domain.Action) error {
	if slices.Contains(entry.Actions, action) {
		return nil
	}

	return domain.NewValidationError(
		"permissions",
		fmt.Sprintf("action %q is not allowed for object %q", action, entry.Object),
	)
}

// validateAssignmentDomain enforces that the assignment's domain matches the
// shape required by the object's scope. The exhaustive switch is deliberate:
// when a new ObjectScope is introduced, the compiler will flag this site as
// the single place that must learn the new shape rule.
func validateAssignmentDomain(scope filter.ObjectScope, obj domain.Object, dom string) error {
	switch scope {
	case filter.ScopeGlobal:
		return validateGlobalDomain(obj, dom)
	case filter.ScopeNamespace:
		return validatePrefixedDomain(obj, dom, domain.NamespaceResourcePrefix, "<name>")
	case filter.ScopeGroup:
		return validatePrefixedDomain(obj, dom, domain.GroupResourcePrefix, "<name>")
	case filter.ScopeUnspecified:
		return domain.NewValidationError(
			"permissions",
			fmt.Sprintf("object %q has an unspecified scope", obj),
		)
	}

	return nil
}

func validateGlobalDomain(obj domain.Object, dom string) error {
	if dom == domain.DomainAll {
		return nil
	}

	return domain.NewValidationError(
		"permissions",
		fmt.Sprintf(
			"object %q is global; domain must be %q, got %q",
			obj, domain.DomainAll, dom,
		),
	)
}

// validatePrefixedDomain enforces the "<prefix><name>" or DomainAll shape for
// resource-scoped objects (Namespace, Group). idPlaceholder is the human-form
// label of the suffix used in error messages, e.g. "<name>".
func validatePrefixedDomain(obj domain.Object, dom, prefix, idPlaceholder string) error {
	if dom == "" {
		return domain.NewValidationError(
			"permissions",
			fmt.Sprintf(
				"object %q requires a %s domain (use %q for wildcard)",
				obj, strings.TrimSuffix(prefix, ":"), domain.DomainAll,
			),
		)
	}
	if dom == domain.DomainAll || strings.HasPrefix(dom, prefix) {
		return nil
	}

	return domain.NewValidationError(
		"permissions",
		fmt.Sprintf(
			"object %q domain must be %q or %q, got %q",
			obj, domain.DomainAll, prefix+idPlaceholder, dom,
		),
	)
}

// cascadeCheckPerms asserts that if the group has members, the actor holds
// every permission the group will hold post-update — because the change
// cascades to existing members through their membership g-rules. Called
// only when an effective delta exists (no-op deltas skip this check).
func (s *Service) cascadeCheckPerms(
	actor domain.AuthInfo,
	groupName string,
	post []domain.Permission,
) error {
	if len(s.pap.GroupMembers(groupName)) == 0 {
		return nil
	}
	for _, p := range post {
		if !s.pdp.Has(actor.UserID, p) {
			return domain.ErrPermissionEscalation
		}
	}

	return nil
}
