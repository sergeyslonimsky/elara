//go:build integration

package config_test

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1/configv1connect"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	itest "github.com/sergeyslonimsky/elara/test/integration"
)

// NOTE: WatchConfigs (server-streaming) is not covered by this table-driven harness —
// streaming requires a different driver. History/Diff/Search/Validate/Copy/GetAtRevision/
// Lock/Unlock are intentionally out of scope here; they should land in a follow-up pass.

type configCase struct {
	name     string
	user     string
	reqPath  string
	respPath string
}

func runConfigCase(t *testing.T, endpoint string, tc configCase) {
	t.Helper()

	app := itest.New(t)

	if tc.user == "admin" {
		user, err := app.Adapters.AuthUsers.GetByEmail(t.Context(), "carol@example.com")
		require.NoError(t, err)
		user.PasswordChangeRequired = false
		require.NoError(t, app.Adapters.AuthUsers.Update(t.Context(), user))
	}

	// EL-50: namespace domains are now prefixed with "namespace:".
	// Suite.New seeds them without prefix, so we re-seed for the personas we use.
	if tc.user == "devops" || tc.user == "tester" || tc.user == "developer" {
		require.NoError(t, app.Managers.Enforcer.WriteTx(t.Context(), app.Adapters.StorageManager, func(ctx context.Context, txe *casbin.TxEnforcer) error {
			for _, p := range itest.DefaultGroupPermissions {
				if p.Group == tc.user {
					// Add prefixed version of the policy
					if err := txe.AddPolicy(
						domain.GroupResource(p.Group),
						domain.NamespaceResource(p.Domain),
						string(p.Object),
						string(p.Action),
					); err != nil {
						return err
					}
				}
			}
			return nil
		}))
		require.NoError(t, app.Managers.Enforcer.LoadPolicy())
	}

	reqBody := itest.ReadFile(t, tc.reqPath)

	resp := itest.DoRequest(t, app, endpoint, reqBody, itest.WithPersona(app, tc.user))
	defer func() { require.NoError(t, resp.Body.Close()) }()

	gotBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	itest.CompareJSONBytes(t, itest.ReadFile(t, tc.respPath), gotBody)
}

func TestIntegration_CreateConfig(t *testing.T) {
	tests := []configCase{
		{
			name:     "admin creates config in dev",
			user:     "admin",
			reqPath:  "testdata/create/admin_ok_req.json",
			respPath: "testdata/create/admin_ok_resp.json",
		},
		{
			name:     "devops writes in prod",
			user:     "devops",
			reqPath:  "testdata/create/devops_prod_ok_req.json",
			respPath: "testdata/create/devops_prod_ok_resp.json",
		},
		{
			name:     "tester denied — reader has no config:write",
			user:     "tester",
			reqPath:  "testdata/create/tester_denied_req.json",
			respPath: "testdata/create/tester_denied_resp.json",
		},
		{
			name:     "unauthenticated denied",
			user:     "unauthenticated",
			reqPath:  "testdata/create/unauthenticated_req.json",
			respPath: "testdata/create/unauthenticated_resp.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runConfigCase(t, configv1connect.ConfigServiceCreateConfigProcedure, tc)
		})
	}
}

func TestIntegration_GetConfig(t *testing.T) {
	tests := []configCase{
		{
			name:     "admin gets seeded config",
			user:     "admin",
			reqPath:  "testdata/get/admin_ok_req.json",
			respPath: "testdata/get/admin_ok_resp.json",
		},
		{
			name:     "tester reads prod",
			user:     "tester",
			reqPath:  "testdata/get/tester_ok_req.json",
			respPath: "testdata/get/tester_ok_resp.json",
		},
		{
			name:     "no-access denied",
			user:     "no-access",
			reqPath:  "testdata/get/no_access_denied_req.json",
			respPath: "testdata/get/no_access_denied_resp.json",
		},
		{
			name:     "admin not found",
			user:     "admin",
			reqPath:  "testdata/get/not_found_req.json",
			respPath: "testdata/get/not_found_resp.json",
		},
		{
			name:     "unauthenticated denied",
			user:     "unauthenticated",
			reqPath:  "testdata/get/unauthenticated_req.json",
			respPath: "testdata/get/unauthenticated_resp.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runConfigCase(t, configv1connect.ConfigServiceGetConfigProcedure, tc)
		})
	}
}

func TestIntegration_UpdateConfig(t *testing.T) {
	tests := []configCase{
		{
			name:     "admin updates prod config",
			user:     "admin",
			reqPath:  "testdata/update/admin_ok_req.json",
			respPath: "testdata/update/admin_ok_resp.json",
		},
		{
			name:     "tester denied — reader has no config:write",
			user:     "tester",
			reqPath:  "testdata/update/tester_denied_req.json",
			respPath: "testdata/update/tester_denied_resp.json",
		},
		{
			name:     "unauthenticated denied",
			user:     "unauthenticated",
			reqPath:  "testdata/update/unauthenticated_req.json",
			respPath: "testdata/update/unauthenticated_resp.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runConfigCase(t, configv1connect.ConfigServiceUpdateConfigProcedure, tc)
		})
	}
}

func TestIntegration_DeleteConfig(t *testing.T) {
	tests := []configCase{
		{
			name:     "admin deletes prod config",
			user:     "admin",
			reqPath:  "testdata/delete/admin_ok_req.json",
			respPath: "testdata/delete/admin_ok_resp.json",
		},
		{
			name:     "tester denied",
			user:     "tester",
			reqPath:  "testdata/delete/tester_denied_req.json",
			respPath: "testdata/delete/tester_denied_resp.json",
		},
		{
			name:     "unauthenticated denied",
			user:     "unauthenticated",
			reqPath:  "testdata/delete/unauthenticated_req.json",
			respPath: "testdata/delete/unauthenticated_resp.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runConfigCase(t, configv1connect.ConfigServiceDeleteConfigProcedure, tc)
		})
	}
}

func TestIntegration_ListConfigs(t *testing.T) {
	tests := []configCase{
		{
			name:     "admin lists prod",
			user:     "admin",
			reqPath:  "testdata/list/admin_prod_req.json",
			respPath: "testdata/list/admin_prod_resp.json",
		},
		{
			name:     "tester reads prod listing",
			user:     "tester",
			reqPath:  "testdata/list/tester_prod_req.json",
			respPath: "testdata/list/tester_prod_resp.json",
		},
		{
			// service_list silently filters: no permission → empty result, not an error.
			name:     "no-access sees empty result",
			user:     "no-access",
			reqPath:  "testdata/list/no_access_denied_req.json",
			respPath: "testdata/list/no_access_denied_resp.json",
		},
		{
			name:     "unauthenticated denied",
			user:     "unauthenticated",
			reqPath:  "testdata/list/unauthenticated_req.json",
			respPath: "testdata/list/unauthenticated_resp.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runConfigCase(t, configv1connect.ConfigServiceListConfigsProcedure, tc)
		})
	}
}
