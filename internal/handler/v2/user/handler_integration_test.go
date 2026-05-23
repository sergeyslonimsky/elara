//go:build integration

package user_test

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/proto/elara/user/v1/userv1connect"
	itest "github.com/sergeyslonimsky/elara/test/integration"
)

// NOTE: OIDC-mode handler branches (h.authType != BasicAuth) are not covered here —
// service.NewServiceManager eagerly discovers the OIDC provider at boot, so flipping
// the auth type via WithConfigOverride is not enough; an OIDC mock server is required.
// Tracked as a framework gap.

type userCase struct {
	name     string
	user     string
	reqPath  string
	respPath string
}

func runUserCase(t *testing.T, endpoint string, tc userCase) {
	t.Helper()

	app := itest.New(t)
	reqBody := itest.ReadFile(t, tc.reqPath)

	resp := itest.DoRequest(t, app, endpoint, reqBody, itest.WithPersona(app, tc.user))
	defer func() { require.NoError(t, resp.Body.Close()) }()

	gotBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	itest.CompareJSONBytes(t, itest.ReadFile(t, tc.respPath), gotBody)
}

func TestIntegration_ListUsers(t *testing.T) {
	tests := []userCase{
		{
			name:     "admin sees empty list",
			user:     "admin",
			reqPath:  "testdata/list_users/admin_empty_req.json",
			respPath: "testdata/list_users/admin_empty_resp.json",
		},
		{
			name:     "devops forbidden",
			user:     "devops",
			reqPath:  "testdata/list_users/devops_forbidden_req.json",
			respPath: "testdata/list_users/devops_forbidden_resp.json",
		},
		{
			name:     "no-access forbidden",
			user:     "no-access",
			reqPath:  "testdata/list_users/no_access_forbidden_req.json",
			respPath: "testdata/list_users/no_access_forbidden_resp.json",
		},
		{
			name:     "unauthenticated denied",
			user:     "unauthenticated",
			reqPath:  "testdata/list_users/unauthenticated_req.json",
			respPath: "testdata/list_users/unauthenticated_resp.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runUserCase(t, userv1connect.UserServiceListUsersProcedure, tc)
		})
	}
}

func TestIntegration_GetUser(t *testing.T) {
	tests := []userCase{
		{
			name:     "admin gets non-existent user",
			user:     "admin",
			reqPath:  "testdata/get_user/admin_not_found_req.json",
			respPath: "testdata/get_user/admin_not_found_resp.json",
		},
		{
			name:     "unauthenticated denied",
			user:     "unauthenticated",
			reqPath:  "testdata/get_user/unauthenticated_req.json",
			respPath: "testdata/get_user/unauthenticated_resp.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runUserCase(t, userv1connect.UserServiceGetUserProcedure, tc)
		})
	}
}

func TestIntegration_CreateUser(t *testing.T) {
	tests := []userCase{
		{
			name:     "admin creates user (basic-auth)",
			user:     "admin",
			reqPath:  "testdata/create_user/admin_ok_req.json",
			respPath: "testdata/create_user/admin_ok_resp.json",
		},
		{
			name:     "admin basic-auth without password",
			user:     "admin",
			reqPath:  "testdata/create_user/admin_no_password_req.json",
			respPath: "testdata/create_user/admin_no_password_resp.json",
		},
		{
			name:     "unauthenticated denied",
			user:     "unauthenticated",
			reqPath:  "testdata/create_user/unauthenticated_req.json",
			respPath: "testdata/create_user/unauthenticated_resp.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runUserCase(t, userv1connect.UserServiceCreateUserProcedure, tc)
		})
	}
}

func TestIntegration_ResetUserPassword(t *testing.T) {
	tests := []userCase{
		{
			name:     "admin resets non-existent user",
			user:     "admin",
			reqPath:  "testdata/reset_password/admin_not_found_req.json",
			respPath: "testdata/reset_password/admin_not_found_resp.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runUserCase(t, userv1connect.UserServiceResetUserPasswordProcedure, tc)
		})
	}
}

func TestIntegration_DeleteUser(t *testing.T) {
	tests := []userCase{
		{
			name:     "admin cannot delete self",
			user:     "admin",
			reqPath:  "testdata/delete_user/admin_self_delete_req.json",
			respPath: "testdata/delete_user/admin_self_delete_resp.json",
		},
		{
			name:     "admin deletes non-existent",
			user:     "admin",
			reqPath:  "testdata/delete_user/admin_not_found_req.json",
			respPath: "testdata/delete_user/admin_not_found_resp.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runUserCase(t, userv1connect.UserServiceDeleteUserProcedure, tc)
		})
	}
}

// TestIntegration_UserLifecycle exercises the framework with a multi-step scenario:
// create → list → get → reset password → delete → verify gone.
// One sub-test, no parallelism (mutates the same store sequentially).
func TestIntegration_UserLifecycle(t *testing.T) {
	t.Parallel()

	app := itest.New(t)
	admin := itest.WithPersona(app, "admin")

	// 1. Create user
	resp := itest.DoRequest(t, app,
		userv1connect.UserServiceCreateUserProcedure,
		itest.ReadFile(t, "testdata/lifecycle/01_create_req.json"),
		admin,
	)
	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	itest.CompareJSONBytes(t, itest.ReadFile(t, "testdata/lifecycle/01_create_resp.json"), got)

	// 2. List shows the new user
	resp = itest.DoRequest(t, app,
		userv1connect.UserServiceListUsersProcedure,
		itest.ReadFile(t, "testdata/lifecycle/02_list_req.json"),
		admin,
	)
	got, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	itest.CompareJSONBytes(t, itest.ReadFile(t, "testdata/lifecycle/02_list_resp.json"), got)

	// 3. Get fetches the new user
	resp = itest.DoRequest(t, app,
		userv1connect.UserServiceGetUserProcedure,
		itest.ReadFile(t, "testdata/lifecycle/03_get_req.json"),
		admin,
	)
	got, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	itest.CompareJSONBytes(t, itest.ReadFile(t, "testdata/lifecycle/03_get_resp.json"), got)

	// 4. Reset password succeeds
	resp = itest.DoRequest(t, app,
		userv1connect.UserServiceResetUserPasswordProcedure,
		itest.ReadFile(t, "testdata/lifecycle/04_reset_req.json"),
		admin,
	)
	got, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	itest.CompareJSONBytes(t, itest.ReadFile(t, "testdata/lifecycle/04_reset_resp.json"), got)

	// 5. Delete user
	resp = itest.DoRequest(t, app,
		userv1connect.UserServiceDeleteUserProcedure,
		itest.ReadFile(t, "testdata/lifecycle/05_delete_req.json"),
		admin,
	)
	got, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	itest.CompareJSONBytes(t, itest.ReadFile(t, "testdata/lifecycle/05_delete_resp.json"), got)

	// 6. Get the deleted user — not found
	resp = itest.DoRequest(t, app,
		userv1connect.UserServiceGetUserProcedure,
		itest.ReadFile(t, "testdata/lifecycle/06_get_after_delete_req.json"),
		admin,
	)
	got, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	itest.CompareJSONBytes(t, itest.ReadFile(t, "testdata/lifecycle/06_get_after_delete_resp.json"), got)
}
