package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
)

// Bootstrap performs all one-shot writes that depend on configuration but not
// on any service: seeding the casbin policy, ensuring the basic-auth admin
// user exists, granting the configured admin email full privileges. Safe to
// call on every startup — each step is idempotent.
func Bootstrap(
	ctx context.Context,
	a *Adapters,
	cfg config.Config,
	enforcer *casbin.Enforcer,
) error {
	if !cfg.UI.Auth.Enabled {
		if err := enforcer.SeedPassthroughAdmin(); err != nil {
			return fmt.Errorf("seed passthrough admin: %w", err)
		}

		return nil
	}

	if cfg.UI.Auth.Type == config.AuthTypeBasicAuth {
		if err := bootstrapBasicAuthAdmin(ctx, a, cfg); err != nil {
			return err
		}
	}

	if email := cfg.UI.Auth.AdminEmail; email != "" {
		if err := enforcer.AddRoleForUser(email, auth.RoleAdmin, auth.ObjectAll); err != nil {
			return fmt.Errorf("bootstrap admin role %q: %w", email, err)
		}

		if err := enforcer.AddPolicy(email, auth.ObjectAll, auth.ObjectAll, auth.ActionAll); err != nil {
			return fmt.Errorf("bootstrap admin policy %q: %w", email, err)
		}
	}

	return nil
}

// bootstrapBasicAuthAdmin creates the initial admin user with the configured
// password if it doesn't already exist.
func bootstrapBasicAuthAdmin(ctx context.Context, a *Adapters, cfg config.Config) error {
	email := cfg.UI.Auth.AdminEmail
	if email == "" {
		return nil
	}

	if _, err := a.AuthUsers.Get(ctx, email); err == nil {
		return nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("check admin user existence: %w", err)
	}

	user := &domain.User{
		Email:    email,
		Name:     "Administrator",
		Provider: domain.ProviderBasicAuth,
	}

	if err := a.AuthUsers.Upsert(ctx, user); err != nil {
		return fmt.Errorf("bootstrap admin upsert: %w", err)
	}

	hash, err := auth.HashPassword(cfg.UI.Auth.BasicAuth.AdminInitialPassword)
	if err != nil {
		return fmt.Errorf("bootstrap admin hash: %w", err)
	}

	if err := a.AuthUsers.SetPassword(ctx, email, hash, true); err != nil {
		return fmt.Errorf("bootstrap admin set password: %w", err)
	}

	return nil
}
