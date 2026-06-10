package auth_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	grouprepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/group"
	policyrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/policy"
	userrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/user"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

// TestAdminBootstrap_PDPProbe verifies that after Bootstrap (local or OIDC),
// a PDP constructed over the same store immediately sees the superadmin
// privilege. This confirms that Bootstrap's direct-adapter writes are
// compatible with the enforcer's expected schema.
func TestAdminBootstrap_PDPProbe(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "probe.db")

	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	storageManager := bbolt.NewManager(store.DB())
	pkgManager := pkgbbolt.NewManager(store.DB())
	users := userrepo.NewRepository(pkgManager)
	groups := grouprepo.NewRepository(pkgManager)
	policies := policyrepo.NewRepository(pkgManager)

	userSvc := auth.NewUserService(users)
	bs := auth.NewAdminBootstrap(storageManager, userSvc, groups, policies)

	// Seed basic admin.
	adminEmail := "admin@elara.internal"
	require.NoError(t, bs.BootstrapBasic(ctx, adminEmail, "pw"))

	// Construct PDP over the SAME store.
	enforcer, err := casbin.NewEnforcer(policies)
	require.NoError(t, err)
	pdp := authz.NewPDP(enforcer)

	// Bootstrap stores the superadmin policy under user.ID (UUID), not email.
	// Resolve the seeded user to get the minted UUID.
	seededUser, err := users.GetByIdentity(ctx, string(domain.ProviderBasic), adminEmail)
	require.NoError(t, err)

	// Admin must have wildcard.
	ok := pdp.Has(seededUser.ID.String(), domain.Permission{
		Object: "anything",
		Action: "delete",
		Domain: "prod",
	})
	assert.True(t, ok, "PDP must see superadmin wildcard seeded by bootstrap")
}
