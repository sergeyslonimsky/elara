//go:build integration

package dashboard_test

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/proto/elara/dashboard/v1/dashboardv1connect"
	itest "github.com/sergeyslonimsky/elara/test/integration"
)

type dashboardCase struct {
	name     string
	user     string
	reqPath  string
	respPath string
}

func runDashboardCase(t *testing.T, endpoint string, tc dashboardCase) {
	t.Helper()

	app := itest.New(t)
	reqBody := itest.ReadFile(t, tc.reqPath)

	resp := itest.DoRequest(t, app, endpoint, reqBody, itest.WithPersona(app, tc.user))
	defer func() { require.NoError(t, resp.Body.Close()) }()

	gotBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	itest.CompareJSONBytes(t, itest.ReadFile(t, tc.respPath), gotBody)
}

func TestIntegration_GetStats(t *testing.T) {
	tests := []dashboardCase{
		{
			name:     "admin sees all 3 namespaces and 6 configs",
			user:     "admin",
			reqPath:  "testdata/stats/admin_req.json",
			respPath: "testdata/stats/admin_resp.json",
		},
		{
			name:     "tester sees only prod (1 ns, 2 configs)",
			user:     "tester",
			reqPath:  "testdata/stats/tester_req.json",
			respPath: "testdata/stats/tester_resp.json",
		},
		{
			name:     "developer sees dev + staging (2 ns, 4 configs)",
			user:     "developer",
			reqPath:  "testdata/stats/developer_req.json",
			respPath: "testdata/stats/developer_resp.json",
		},
		{
			name:     "no-access sees zero counts (no error)",
			user:     "no-access",
			reqPath:  "testdata/stats/no_access_req.json",
			respPath: "testdata/stats/no_access_resp.json",
		},
		{
			name:     "unauthenticated denied",
			user:     "unauthenticated",
			reqPath:  "testdata/stats/unauthenticated_req.json",
			respPath: "testdata/stats/unauthenticated_resp.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runDashboardCase(t, dashboardv1connect.DashboardServiceGetStatsProcedure, tc)
		})
	}
}

func TestIntegration_ListActivity(t *testing.T) {
	tests := []dashboardCase{
		{
			name:     "admin lists activity",
			user:     "admin",
			reqPath:  "testdata/activity/admin_req.json",
			respPath: "testdata/activity/admin_resp.json",
		},
		{
			name:     "tester lists activity (scoped to prod)",
			user:     "tester",
			reqPath:  "testdata/activity/tester_req.json",
			respPath: "testdata/activity/tester_resp.json",
		},
		{
			name:     "no-access sees empty (no error)",
			user:     "no-access",
			reqPath:  "testdata/activity/no_access_req.json",
			respPath: "testdata/activity/no_access_resp.json",
		},
		{
			name:     "unauthenticated denied",
			user:     "unauthenticated",
			reqPath:  "testdata/activity/unauthenticated_req.json",
			respPath: "testdata/activity/unauthenticated_resp.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runDashboardCase(t, dashboardv1connect.DashboardServiceListActivityProcedure, tc)
		})
	}
}
