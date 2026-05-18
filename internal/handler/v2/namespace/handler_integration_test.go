//go:build integration

package namespace_test

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/proto/elara/namespace/v1/namespacev1connect"
	itest "github.com/sergeyslonimsky/elara/test/integration"
)

func TestIntegration_CreateNamespace(t *testing.T) {
	endpoint := namespacev1connect.NamespaceServiceCreateNamespaceProcedure

	tests := []struct {
		name     string
		user     string
		reqPath  string
		respPath string
	}{
		{
			name:     "admin creates namespace",
			user:     "admin",
			reqPath:  "testdata/create/admin_ok_req.json",
			respPath: "testdata/create/admin_ok_resp.json",
		},
		{
			name:     "devops denied — writer has no namespace:write",
			user:     "devops",
			reqPath:  "testdata/create/writer_denied_req.json",
			respPath: "testdata/create/writer_denied_resp.json",
		},
		{
			name:     "tester denied — reader has no namespace:write",
			user:     "tester",
			reqPath:  "testdata/create/reader_denied_req.json",
			respPath: "testdata/create/reader_denied_resp.json",
		},
		{
			name:     "unauthenticated denied",
			user:     "unauthenticated",
			reqPath:  "testdata/create/unauthenticated_req.json",
			respPath: "testdata/create/unauthenticated_resp.json",
		},
		{
			name:     "invalid name rejected",
			user:     "admin",
			reqPath:  "testdata/create/invalid_name_req.json",
			respPath: "testdata/create/invalid_name_resp.json",
		},
		{
			name:     "duplicate name rejected",
			user:     "admin",
			reqPath:  "testdata/create/duplicate_req.json",
			respPath: "testdata/create/duplicate_resp.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := itest.New(t)
			reqBody := itest.ReadFile(t, tc.reqPath)

			resp := itest.DoRequest(t, app, endpoint, reqBody, itest.WithPersona(app, tc.user))
			defer func() { require.NoError(t, resp.Body.Close()) }()

			gotBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			itest.CompareJSONBytes(t, itest.ReadFile(t, tc.respPath), gotBody)
		})
	}
}

func TestIntegration_GetNamespace(t *testing.T) {
	endpoint := namespacev1connect.NamespaceServiceGetNamespaceProcedure

	tests := []struct {
		name     string
		user     string
		reqPath  string
		respPath string
	}{
		{
			name:     "admin gets prod",
			user:     "admin",
			reqPath:  "testdata/get/admin_prod_req.json",
			respPath: "testdata/get/admin_prod_resp.json",
		},
		{
			name:     "devops gets prod",
			user:     "devops",
			reqPath:  "testdata/get/writer_prod_ok_req.json",
			respPath: "testdata/get/writer_prod_ok_resp.json",
		},
		{
			name:     "devops cannot get staging",
			user:     "devops",
			reqPath:  "testdata/get/writer_staging_denied_req.json",
			respPath: "testdata/get/writer_staging_denied_resp.json",
		},
		{
			name:     "tester gets prod",
			user:     "tester",
			reqPath:  "testdata/get/reader_prod_ok_req.json",
			respPath: "testdata/get/reader_prod_ok_resp.json",
		},
		{
			name:     "no-access denied",
			user:     "no-access",
			reqPath:  "testdata/get/no_access_denied_req.json",
			respPath: "testdata/get/no_access_denied_resp.json",
		},
		{
			name:     "unauthenticated denied",
			user:     "unauthenticated",
			reqPath:  "testdata/get/unauthenticated_req.json",
			respPath: "testdata/get/unauthenticated_resp.json",
		},
		{
			name:     "admin not found",
			user:     "admin",
			reqPath:  "testdata/get/not_found_req.json",
			respPath: "testdata/get/not_found_resp.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := itest.New(t)
			reqBody := itest.ReadFile(t, tc.reqPath)

			resp := itest.DoRequest(t, app, endpoint, reqBody, itest.WithPersona(app, tc.user))
			defer func() { require.NoError(t, resp.Body.Close()) }()

			gotBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			itest.CompareJSONBytes(t, itest.ReadFile(t, tc.respPath), gotBody)
		})
	}
}

func TestIntegration_UpdateNamespace(t *testing.T) {
	endpoint := namespacev1connect.NamespaceServiceUpdateNamespaceProcedure

	tests := []struct {
		name     string
		user     string
		reqPath  string
		respPath string
	}{
		{
			name:     "admin updates prod",
			user:     "admin",
			reqPath:  "testdata/update/admin_ok_req.json",
			respPath: "testdata/update/admin_ok_resp.json",
		},
		{
			name:     "devops denied",
			user:     "devops",
			reqPath:  "testdata/update/writer_denied_req.json",
			respPath: "testdata/update/writer_denied_resp.json",
		},
		{
			name:     "tester denied",
			user:     "tester",
			reqPath:  "testdata/update/reader_denied_req.json",
			respPath: "testdata/update/reader_denied_resp.json",
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

			app := itest.New(t)
			reqBody := itest.ReadFile(t, tc.reqPath)

			resp := itest.DoRequest(t, app, endpoint, reqBody, itest.WithPersona(app, tc.user))
			defer func() { require.NoError(t, resp.Body.Close()) }()

			gotBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			itest.CompareJSONBytes(t, itest.ReadFile(t, tc.respPath), gotBody)
		})
	}
}

func TestIntegration_ListNamespaces(t *testing.T) {
	endpoint := namespacev1connect.NamespaceServiceListNamespacesProcedure

	tests := []struct {
		name     string
		user     string
		reqPath  string
		respPath string
	}{
		{
			name:     "admin sees all",
			user:     "admin",
			reqPath:  "testdata/list/admin_all_req.json",
			respPath: "testdata/list/admin_all_resp.json",
		},
		{
			name:     "devops sees prod only",
			user:     "devops",
			reqPath:  "testdata/list/writer_prod_only_req.json",
			respPath: "testdata/list/writer_prod_only_resp.json",
		},
		{
			name:     "tester sees prod only",
			user:     "tester",
			reqPath:  "testdata/list/reader_prod_only_req.json",
			respPath: "testdata/list/reader_prod_only_resp.json",
		},
		{
			name:     "developer sees dev and staging",
			user:     "developer",
			reqPath:  "testdata/list/developer_dev_staging_req.json",
			respPath: "testdata/list/developer_dev_staging_resp.json",
		},
		{
			name:     "no-access sees empty",
			user:     "no-access",
			reqPath:  "testdata/list/no_access_empty_req.json",
			respPath: "testdata/list/no_access_empty_resp.json",
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

			app := itest.New(t)
			reqBody := itest.ReadFile(t, tc.reqPath)

			resp := itest.DoRequest(t, app, endpoint, reqBody, itest.WithPersona(app, tc.user))
			defer func() { require.NoError(t, resp.Body.Close()) }()

			gotBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			itest.CompareJSONBytes(t, itest.ReadFile(t, tc.respPath), gotBody)
		})
	}
}

func TestIntegration_DeleteNamespace(t *testing.T) {
	endpoint := namespacev1connect.NamespaceServiceDeleteNamespaceProcedure

	tests := []struct {
		name     string
		user     string
		reqPath  string
		respPath string
	}{
		{
			name:     "admin blocked when namespace has configs",
			user:     "admin",
			reqPath:  "testdata/delete/admin_has_configs_req.json",
			respPath: "testdata/delete/admin_has_configs_resp.json",
		},
		{
			name:     "admin deletes missing namespace",
			user:     "admin",
			reqPath:  "testdata/delete/admin_not_found_req.json",
			respPath: "testdata/delete/admin_not_found_resp.json",
		},
		{
			name:     "devops denied",
			user:     "devops",
			reqPath:  "testdata/delete/writer_denied_req.json",
			respPath: "testdata/delete/writer_denied_resp.json",
		},
		{
			name:     "tester denied",
			user:     "tester",
			reqPath:  "testdata/delete/reader_denied_req.json",
			respPath: "testdata/delete/reader_denied_resp.json",
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

			app := itest.New(t)
			reqBody := itest.ReadFile(t, tc.reqPath)

			resp := itest.DoRequest(t, app, endpoint, reqBody, itest.WithPersona(app, tc.user))
			defer func() { require.NoError(t, resp.Body.Close()) }()

			gotBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			itest.CompareJSONBytes(t, itest.ReadFile(t, tc.respPath), gotBody)
		})
	}
}

func TestIntegration_LockNamespace(t *testing.T) {
	endpoint := namespacev1connect.NamespaceServiceLockNamespaceProcedure

	tests := []struct {
		name     string
		user     string
		reqPath  string
		respPath string
	}{
		{
			name:     "admin locks prod",
			user:     "admin",
			reqPath:  "testdata/lock/admin_ok_req.json",
			respPath: "testdata/lock/admin_ok_resp.json",
		},
		{
			name:     "devops denied",
			user:     "devops",
			reqPath:  "testdata/lock/writer_denied_req.json",
			respPath: "testdata/lock/writer_denied_resp.json",
		},
		{
			name:     "tester denied",
			user:     "tester",
			reqPath:  "testdata/lock/reader_denied_req.json",
			respPath: "testdata/lock/reader_denied_resp.json",
		},
		{
			name:     "unauthenticated denied",
			user:     "unauthenticated",
			reqPath:  "testdata/lock/unauthenticated_req.json",
			respPath: "testdata/lock/unauthenticated_resp.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := itest.New(t)
			reqBody := itest.ReadFile(t, tc.reqPath)

			resp := itest.DoRequest(t, app, endpoint, reqBody, itest.WithPersona(app, tc.user))
			defer func() { require.NoError(t, resp.Body.Close()) }()

			gotBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			itest.CompareJSONBytes(t, itest.ReadFile(t, tc.respPath), gotBody)
		})
	}
}

func TestIntegration_UnlockNamespace(t *testing.T) {
	endpoint := namespacev1connect.NamespaceServiceUnlockNamespaceProcedure

	tests := []struct {
		name     string
		user     string
		reqPath  string
		respPath string
	}{
		{
			name:     "admin unlocks prod",
			user:     "admin",
			reqPath:  "testdata/unlock/admin_ok_req.json",
			respPath: "testdata/unlock/admin_ok_resp.json",
		},
		{
			name:     "devops denied",
			user:     "devops",
			reqPath:  "testdata/unlock/writer_denied_req.json",
			respPath: "testdata/unlock/writer_denied_resp.json",
		},
		{
			name:     "tester denied",
			user:     "tester",
			reqPath:  "testdata/unlock/reader_denied_req.json",
			respPath: "testdata/unlock/reader_denied_resp.json",
		},
		{
			name:     "unauthenticated denied",
			user:     "unauthenticated",
			reqPath:  "testdata/unlock/unauthenticated_req.json",
			respPath: "testdata/unlock/unauthenticated_resp.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := itest.New(t)
			reqBody := itest.ReadFile(t, tc.reqPath)

			resp := itest.DoRequest(t, app, endpoint, reqBody, itest.WithPersona(app, tc.user))
			defer func() { require.NoError(t, resp.Body.Close()) }()

			gotBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			itest.CompareJSONBytes(t, itest.ReadFile(t, tc.respPath), gotBody)
		})
	}
}
