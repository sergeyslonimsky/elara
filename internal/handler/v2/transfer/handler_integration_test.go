//go:build integration

package transfer_test

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1/transferv1connect"
	itest "github.com/sergeyslonimsky/elara/test/integration"
)

func TestIntegration_ExportAll(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		reqPath  string
		respPath string
	}{
		{
			name:     "admin sees all namespaces",
			user:     "admin",
			reqPath:  "testdata/export_all/admin_all_namespaces_req.json",
			respPath: "testdata/export_all/admin_all_namespaces_resp.json",
		},
		{
			name:     "devops sees only prod",
			user:     "devops",
			reqPath:  "testdata/export_all/writer_prod_filtered_req.json",
			respPath: "testdata/export_all/writer_prod_filtered_resp.json",
		},
		{
			name:     "tester sees only prod",
			user:     "tester",
			reqPath:  "testdata/export_all/reader_prod_filtered_req.json",
			respPath: "testdata/export_all/reader_prod_filtered_resp.json",
		},
		{
			name:     "no-access gets empty bundle",
			user:     "no-access",
			reqPath:  "testdata/export_all/no_access_empty_req.json",
			respPath: "testdata/export_all/no_access_empty_resp.json",
		},
		{
			name:     "unauthenticated denied",
			user:     "unauthenticated",
			reqPath:  "testdata/export_all/unauthenticated_req.json",
			respPath: "testdata/export_all/unauthenticated_resp.json",
		},
	}

	endpoint := transferv1connect.TransferServiceExportAllProcedure

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

func TestIntegration_ExportNamespace(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		reqPath  string
		respPath string
	}{
		{
			name:     "admin exports prod",
			user:     "admin",
			reqPath:  "testdata/export_namespace/admin_prod_req.json",
			respPath: "testdata/export_namespace/admin_prod_resp.json",
		},
		{
			name:     "devops exports prod",
			user:     "devops",
			reqPath:  "testdata/export_namespace/writer_prod_ok_req.json",
			respPath: "testdata/export_namespace/writer_prod_ok_resp.json",
		},
		{
			name:     "devops cannot export staging",
			user:     "devops",
			reqPath:  "testdata/export_namespace/writer_prod_staging_denied_req.json",
			respPath: "testdata/export_namespace/writer_prod_staging_denied_resp.json",
		},
		{
			name:     "tester exports prod",
			user:     "tester",
			reqPath:  "testdata/export_namespace/reader_prod_ok_req.json",
			respPath: "testdata/export_namespace/reader_prod_ok_resp.json",
		},
		{
			name:     "tester cannot export staging",
			user:     "tester",
			reqPath:  "testdata/export_namespace/reader_prod_staging_denied_req.json",
			respPath: "testdata/export_namespace/reader_prod_staging_denied_resp.json",
		},
		{
			name:     "no-access denied",
			user:     "no-access",
			reqPath:  "testdata/export_namespace/no_access_denied_req.json",
			respPath: "testdata/export_namespace/no_access_denied_resp.json",
		},
	}

	endpoint := transferv1connect.TransferServiceExportNamespaceProcedure

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

func TestIntegration_ImportAll(t *testing.T) {
	tests := []struct {
		name       string
		user       string
		reqPath    string
		bundlePath string
		respPath   string
	}{
		{
			name:       "admin can import all (dry run)",
			user:       "admin",
			reqPath:    "testdata/import_all/admin_ok_req.json",
			bundlePath: "testdata/import_all/admin_ok_bundle.json",
			respPath:   "testdata/import_all/admin_ok_resp.json",
		},
		{
			name:       "devops denied — not admin",
			user:       "devops",
			reqPath:    "testdata/import_all/writer_denied_req.json",
			bundlePath: "testdata/import_all/writer_denied_bundle.json",
			respPath:   "testdata/import_all/writer_denied_resp.json",
		},
		{
			name:       "tester denied",
			user:       "tester",
			reqPath:    "testdata/import_all/reader_denied_req.json",
			bundlePath: "testdata/import_all/reader_denied_bundle.json",
			respPath:   "testdata/import_all/reader_denied_resp.json",
		},
	}

	endpoint := transferv1connect.TransferServiceImportNamespaceProcedure

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := itest.New(t)
			reqBody := itest.InjectFileAsBase64(
				t,
				itest.ReadFile(t, tc.reqPath),
				"data",
				tc.bundlePath,
			)

			resp := itest.DoRequest(t, app, endpoint, reqBody, itest.WithPersona(app, tc.user))
			defer func() { require.NoError(t, resp.Body.Close()) }()

			gotBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			itest.CompareJSONBytes(t, itest.ReadFile(t, tc.respPath), gotBody)
		})
	}
}

func TestIntegration_ImportNamespace(t *testing.T) {
	tests := []struct {
		name       string
		user       string
		reqPath    string
		bundlePath string
		respPath   string
	}{
		{
			name:       "admin imports into prod",
			user:       "admin",
			reqPath:    "testdata/import_namespace/admin_prod_ok_req.json",
			bundlePath: "testdata/import_namespace/admin_prod_ok_bundle.json",
			respPath:   "testdata/import_namespace/admin_prod_ok_resp.json",
		},
		{
			name:       "admin imports into staging",
			user:       "admin",
			reqPath:    "testdata/import_namespace/admin_staging_ok_req.json",
			bundlePath: "testdata/import_namespace/admin_staging_ok_bundle.json",
			respPath:   "testdata/import_namespace/admin_staging_ok_resp.json",
		},
		{
			name:       "devops imports into prod",
			user:       "devops",
			reqPath:    "testdata/import_namespace/writer_prod_ok_req.json",
			bundlePath: "testdata/import_namespace/writer_prod_ok_bundle.json",
			respPath:   "testdata/import_namespace/writer_prod_ok_resp.json",
		},
		{
			name:       "devops cannot import into staging",
			user:       "devops",
			reqPath:    "testdata/import_namespace/writer_staging_denied_req.json",
			bundlePath: "testdata/import_namespace/writer_staging_denied_bundle.json",
			respPath:   "testdata/import_namespace/writer_staging_denied_resp.json",
		},
		{
			name:       "tester cannot import into prod",
			user:       "tester",
			reqPath:    "testdata/import_namespace/reader_prod_denied_req.json",
			bundlePath: "testdata/import_namespace/reader_prod_denied_bundle.json",
			respPath:   "testdata/import_namespace/reader_prod_denied_resp.json",
		},
	}

	endpoint := transferv1connect.TransferServiceImportNamespaceProcedure

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := itest.New(t)
			reqBody := itest.InjectFileAsBase64(
				t,
				itest.ReadFile(t, tc.reqPath),
				"data",
				tc.bundlePath,
			)

			resp := itest.DoRequest(t, app, endpoint, reqBody, itest.WithPersona(app, tc.user))
			defer func() { require.NoError(t, resp.Body.Close()) }()

			gotBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			itest.CompareJSONBytes(t, itest.ReadFile(t, tc.respPath), gotBody)
		})
	}
}
