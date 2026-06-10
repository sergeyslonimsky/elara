// Package casbin previously hosted CheckBootstrapAdmin, a helper that wrote a
// direct user->admin g-rule for the configured admin email. After the
// groups-only refactor that path is gone — all admin grants flow through the
// system `admins` group via auth.AdminBootstrap. The function is no longer
// available; this file is kept as a placeholder so the package boundary stays
// stable for future helpers.
package casbin
