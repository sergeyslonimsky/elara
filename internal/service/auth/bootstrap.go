package auth

//go:generate mockgen -destination=mocks/service_mock.go -package=auth_mock -source=bootstrap.go

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

// PassthroughEmail is the synthetic user used when UI auth is disabled. It's
// added as a member of the superadmin group during Seed so that "no auth"
// requests still pass enforcement uniformly through the groups model — no
// direct user->role g-rule is needed.
const PassthroughEmail = "local-admin@elara.internal"

// AdminBootstrap manages the system superadmin identity. It is the single place
// that writes root permissions; it ensures the group, the user, and the
// break-glass wildcard policy rule exist and are correctly linked.
type AdminBootstrap struct {
	txm      storage.TxManager
	users    *bbolt.UserRepo
	groups   *bbolt.GroupRepo
	policies *bbolt.PolicyRepo
}

// NewAdminBootstrap returns a bootstrap helper backed by the given repositories.
func NewAdminBootstrap(
	txm storage.TxManager,
	users *bbolt.UserRepo,
	groups *bbolt.GroupRepo,
	policies *bbolt.PolicyRepo,
) *AdminBootstrap {
	return &AdminBootstrap{
		txm:      txm,
		users:    users,
		groups:   groups,
		policies: policies,
	}
}

// Bootstrap runs the full idempotent bootstrap sequence in a single write transaction.
func (a *AdminBootstrap) Bootstrap(ctx context.Context, username, password string) error {
	if err := a.txm.Write(ctx, func(tx storage.Tx) error {
		if err := a.EnsureSuperAdminGroup(ctx, tx); err != nil {
			return err
		}

		if err := a.EnsureSuperAdminUser(ctx, tx, username, password); err != nil {
			return err
		}

		if err := a.EnsureSuperAdminPolicy(ctx, tx); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	return nil
}

// EnsureSuperAdminGroup creates the superadmin group with System=true if it does not
// yet exist.
func (a *AdminBootstrap) EnsureSuperAdminGroup(ctx context.Context, tx storage.Tx) error {
	groups := a.groups.WithTx(tx)

	group, err := groups.FindByName(ctx, domain.SystemGroupSuperAdmin)
	if err == nil {
		if !group.System {
			group.System = true
			if err := groups.Update(ctx, group); err != nil {
				return fmt.Errorf("update superadmin group system flag: %w", err)
			}
		}

		return nil
	}

	if !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("lookup superadmin group: %w", err)
	}

	now := time.Now().UTC()
	group = &domain.Group{
		ID:        uuid.New().String(),
		Name:      domain.SystemGroupSuperAdmin,
		Members:   []string{},
		System:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := groups.Create(ctx, group); err != nil {
		return fmt.Errorf("create superadmin group: %w", err)
	}

	return nil
}

// EnsureSuperAdminUser creates the superadmin user, sets its password if it's
// new, and ensures it is a member of the superadmin group.
func (a *AdminBootstrap) EnsureSuperAdminUser( //nolint:cyclop //refactor
	ctx context.Context,
	tx storage.Tx,
	username, password string,
) error {
	users := a.users.WithTx(tx)
	groups := a.groups.WithTx(tx)

	user, err := users.Get(ctx, username)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("lookup superadmin user: %w", err)
		}

		hash, err := HashPassword(password)
		if err != nil {
			return fmt.Errorf("hash superadmin password: %w", err)
		}

		now := time.Now().UTC()
		user = &domain.User{
			Email:                  username,
			Name:                   "Super Admin",
			Provider:               domain.ProviderBasicAuth,
			PasswordHash:           hash,
			PasswordChangeRequired: true,
			System:                 true,
			Source:                 domain.SourceLocal,
			CreatedAt:              now,
		}

		if err := users.Upsert(ctx, user); err != nil {
			return fmt.Errorf("create superadmin user: %w", err)
		}
	} else if !user.System {
		user.System = true
		if err := users.Upsert(ctx, user); err != nil {
			return fmt.Errorf("update superadmin user system flag: %w", err)
		}
	}

	group, err := groups.FindByName(ctx, domain.SystemGroupSuperAdmin)
	if err != nil {
		return fmt.Errorf("lookup superadmin group for membership: %w", err)
	}

	isMember := slices.Contains(group.Members, username)

	if !isMember {
		if err := group.AddMember(username); err != nil {
			return fmt.Errorf("add superadmin user to group: %w", err)
		}

		if err := groups.Update(ctx, group); err != nil {
			return fmt.Errorf("persist superadmin membership: %w", err)
		}
	}

	// Idempotently ensure the membership g-rule.
	policy := a.policies.WithTx(tx)
	rule := []string{username, domain.GroupSubject(domain.SystemGroupSuperAdmin), domain.MembershipDomain}

	if err := policy.AddPolicy("g", "g", rule); err != nil {
		return fmt.Errorf("add superadmin membership rule: %w", err)
	}

	return nil
}

// EnsureSuperAdminPolicy writes the break-glass wildcard policy rule for the
// superadmin group if it is missing.
func (a *AdminBootstrap) EnsureSuperAdminPolicy(ctx context.Context, tx storage.Tx) error {
	policy := a.policies.WithTx(tx)
	rule := []string{
		domain.GroupSubject(domain.SystemGroupSuperAdmin),
		domain.DomainAll,
		domain.ObjectAll,
		domain.ActionAll,
	}

	if err := policy.AddPolicy("p", "p", rule); err != nil {
		return fmt.Errorf("add superadmin policy rule: %w", err)
	}

	return nil
}

// EnsureMember ensures the given user is a member of the superadmin group.
// It is used to elevate configured admins after OIDC login.
func (a *AdminBootstrap) EnsureMember(ctx context.Context, email string) error {
	if err := a.txm.Write(ctx, func(tx storage.Tx) error {
		groups := a.groups.WithTx(tx)

		group, err := groups.FindByName(ctx, domain.SystemGroupSuperAdmin)
		if err != nil {
			return fmt.Errorf("lookup superadmin group for membership: %w", err)
		}

		isMember := slices.Contains(group.Members, email)

		if !isMember {
			if err := group.AddMember(email); err != nil {
				return fmt.Errorf("add user to group: %w", err)
			}

			if err := groups.Update(ctx, group); err != nil {
				return fmt.Errorf("persist superadmin membership: %w", err)
			}
		}

		// Idempotently ensure the membership g-rule.
		policy := a.policies.WithTx(tx)
		rule := []string{email, domain.GroupSubject(domain.SystemGroupSuperAdmin), domain.MembershipDomain}

		if err := policy.AddPolicy("g", "g", rule); err != nil {
			return fmt.Errorf("add superadmin membership rule: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("ensure superadmin member: %w", err)
	}

	return nil
}
