package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/di/service"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

// Persona is an RBAC identity used in integration test cases. Membership is
// represented by Group — direct role assignments do not exist in the unified
// RBAC model (architecture.md §4: users get permissions only through groups).
type Persona struct {
	Email string
	Group string // group name, joined via g-rule (user, group:<name>, *)
}

// GroupPerm grants (object, action) in a specific domain to a group.
// Translates to Casbin p-rule: p, group:<group>, <domain>, <object>, <action>.
type GroupPerm struct {
	Group  string
	Object domain.Object
	Action domain.Action
	Domain string
}

// DefaultGroupPermissions seeds the standard test groups with permissions on
// the objects exercised by handler integration tests (namespace, config,
// transfer, user, group, token, dashboard, webhook).
var DefaultGroupPermissions = func() []GroupPerm {
	var out []GroupPerm

	// objects every "domain-scoped" group needs read or write on within its namespace(s).
	// ObjectToken is per-namespace as of EL-4 T9.6: token visibility/management
	// is gated by (Token, action, ns), not by Namespace:Read or global IAM perms.
	// Namespace is the umbrella for config/schema/transfer content; webhook and
	// token remain distinct namespace-scoped objects.
	scopedObjects := []domain.Object{
		domain.ObjectNamespace,
		domain.ObjectDashboard,
		domain.ObjectWebhook,
		domain.ObjectToken,
	}

	addGroup := func(group string, action domain.Action, namespaces ...string) {
		for _, ns := range namespaces {
			for _, obj := range scopedObjects {
				out = append(out, GroupPerm{Group: group, Object: obj, Action: action, Domain: ns})
			}
		}
	}

	// devops: read+write+create in prod. (Actions are flat in the unified RBAC
	// model — Write covers update/delete on existing, Create is separate.
	// Mirror the "writer can also create and read" UX expected by fixtures.)
	addGroup("devops", domain.ActionRead, "prod")
	addGroup("devops", domain.ActionWrite, "prod")
	addGroup("devops", domain.ActionCreate, "prod")
	// developer: full r/w/c in dev, read-only in staging — mixed-permission persona.
	addGroup("developer", domain.ActionRead, "dev")
	addGroup("developer", domain.ActionWrite, "dev")
	addGroup("developer", domain.ActionCreate, "dev")
	addGroup("developer", domain.ActionRead, "staging")
	// tester: read in prod only.
	addGroup("tester", domain.ActionRead, "prod")

	// IAM objects (user, group, token) are global — tested via personas that
	// would never need cross-cutting IAM access. Superadmin handles those.

	return out
}()

// DefaultPersonas is the standard set of identities shared across all handler integration tests.
var DefaultPersonas = map[string]Persona{
	"admin":           {Email: "carol@example.com"}, // added to system:superadmin in New()
	"devops":          {Email: "alice@example.com", Group: "devops"},
	"developer":       {Email: "bob@example.com", Group: "developer"},
	"tester":          {Email: "dave@example.com", Group: "tester"},
	"no-access":       {Email: "eve@example.com"},
	"unauthenticated": {},
}

// Suite is a running httptest.Server with persona session tokens pre-issued.
type Suite struct {
	Server  *httptest.Server
	Tokens  map[string]string // persona name → raw session JWT
	Manager *service.Manager
}

const testSessionSecret = "test-integration-secret"

// muxAdapter implements the server interface required by service.V2Routes.
type muxAdapter struct{ mux *http.ServeMux }

func (a *muxAdapter) Mount(pattern string, h http.Handler) { a.mux.Handle(pattern, h) }

// New starts a full Elara service manager backed by a fresh bbolt store in
// t.TempDir(). Standard seed data (prod + staging + dev namespaces with two
// configs each) and DefaultPersonas with their group memberships are wired
// before the server starts. Superadmin (`carol@example.com`) is created by
// the standard bootstrap path so no test needs to re-implement it.
func New(t *testing.T) *Suite {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	cfg := testConfig(t)

	mgr, _, err := service.NewServiceManager(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		cancel()
		_ = mgr.Adapters.Store.Close()
	})

	// Superadmin is created by the standard bootstrap path — same as production.
	require.NoError(t, mgr.Services.AdminBootstrap.BootstrapBasic(
		ctx,
		DefaultPersonas["admin"].Email,
		"unused-password",
	))

	seedData(t, ctx, mgr)

	txm := bbolt.NewTxManager(mgr.Adapters.Store.DB())
	seedRBAC(t, ctx, mgr.Enforcer, txm)

	// Bootstrap and seedRBAC write directly through the policy repo; reload
	// the enforcer cache so subsequent Enforce checks see the new rules.
	require.NoError(t, mgr.Enforcer.LoadPolicy())

	mux := http.NewServeMux()
	service.V2Routes(&muxAdapter{mux}, mgr.V2Handlers, mgr.SessionManager, cfg)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	tokens := make(map[string]string, len(DefaultPersonas))

	for name, p := range DefaultPersonas {
		if p.Email == "" {
			continue
		}

		token, err := mgr.SessionManager.Create(&domain.User{Email: p.Email})
		require.NoError(t, err)
		tokens[name] = token
	}

	return &Suite{Server: srv, Tokens: tokens, Manager: mgr}
}

func testConfig(t *testing.T) config.Config {
	t.Helper()

	return config.Config{
		DataPath: t.TempDir(),
		UI: config.UI{
			Auth: config.UIAuthConfig{
				Enabled: true,
				Type:    config.AuthTypeBasicAuth,
				Session: config.SessionConfig{
					Secret: testSessionSecret,
					TTL:    time.Hour,
				},
			},
		},
		Client: config.Client{
			History:      config.ClientHistory{MaxRecords: 1000, MaxAge: 30 * 24 * time.Hour},
			RecentEvents: config.ClientRecentEvents{Capacity: 100},
		},
		Metrics: config.MetricsConfig{Enabled: false},
		Tracing: config.TracingConfig{Enabled: false},
		Log:     config.LogConfig{Level: "error", Format: "text"},
	}
}

// seedData populates the store with the standard dataset used across all integration tests.
//
// Namespaces: prod, staging, dev
// Configs: /api.json and /db.json in each namespace.
func seedData(t *testing.T, ctx context.Context, mgr *service.Manager) {
	t.Helper()

	for _, ns := range []string{"prod", "staging", "dev"} {
		require.NoError(t, mgr.Adapters.NamespaceRepo.Create(ctx, &domain.Namespace{Name: ns}))
	}

	configs := []domain.Config{
		{Path: "/api.json", Content: `{"host":"api.prod.example.com"}`, Format: domain.FormatJSON, Namespace: "prod"},
		{Path: "/db.json", Content: `{"host":"pg.prod.example.com"}`, Format: domain.FormatJSON, Namespace: "prod"},
		{
			Path:      "/api.json",
			Content:   `{"host":"api.staging.example.com"}`,
			Format:    domain.FormatJSON,
			Namespace: "staging",
		},
		{
			Path:      "/db.json",
			Content:   `{"host":"pg.staging.example.com"}`,
			Format:    domain.FormatJSON,
			Namespace: "staging",
		},
		{Path: "/api.json", Content: `{"host":"api.dev.example.com"}`, Format: domain.FormatJSON, Namespace: "dev"},
		{Path: "/db.json", Content: `{"host":"pg.dev.example.com"}`, Format: domain.FormatJSON, Namespace: "dev"},
	}

	for i := range configs {
		require.NoError(t, mgr.Adapters.ConfigRepo.Create(ctx, &configs[i]))
	}
}

// AddPersona issues a session JWT for a fresh email and seeds the given group
// permissions on a new ad-hoc group. Use this for M9 acceptance tests that
// need finely-scoped personas (e.g. `(Group, Create, *)` only) without
// polluting DefaultGroupPermissions for everyone.
//
// Returns the raw session token to attach via itest.WithToken.
func (s *Suite) AddPersona(t *testing.T, email, group string, perms []GroupPerm) string {
	t.Helper()

	ctx := t.Context()
	txm := bbolt.NewTxManager(s.Manager.Adapters.Store.DB())

	require.NoError(t, s.Manager.Enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		for _, p := range perms {
			if err := txe.AddPolicy(
				casbin.GroupSubject(group),
				p.Domain,
				string(p.Object),
				string(p.Action),
			); err != nil {
				return err
			}
		}

		return txe.AddRoleForUser(email, casbin.GroupSubject(group), domain.MembershipDomain)
	}))
	require.NoError(t, s.Manager.Enforcer.LoadPolicy())

	token, err := s.Manager.SessionManager.Create(&domain.User{Email: email})
	require.NoError(t, err)

	return token
}

// seedRBAC writes DefaultGroupPermissions as Casbin p-rules and binds personas
// to their groups via g-rules. All mutations run inside a single TxManager.Write
// so the test enforcer state matches production atomicity (architecture.md §4 L2).
func seedRBAC(t *testing.T, ctx context.Context, enforcer *casbin.Enforcer, txm storage.TxManager) {
	t.Helper()

	require.NoError(t, enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		for _, p := range DefaultGroupPermissions {
			if err := txe.AddPolicy(
				casbin.GroupSubject(p.Group),
				p.Domain,
				string(p.Object),
				string(p.Action),
			); err != nil {
				return err
			}
		}

		for _, persona := range DefaultPersonas {
			if persona.Email == "" || persona.Group == "" {
				continue
			}

			if err := txe.AddRoleForUser(
				persona.Email,
				casbin.GroupSubject(persona.Group),
				domain.MembershipDomain,
			); err != nil {
				return err
			}
		}

		return nil
	}))
}
