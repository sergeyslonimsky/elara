package transfer_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gopkg.in/yaml.v3"

	"github.com/sergeyslonimsky/elara/internal/domain"
	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
	"github.com/sergeyslonimsky/elara/internal/usecase/transfer"
)

func TestService_ExportAll(t *testing.T) {
	t.Parallel()

	type input struct {
		asZip    bool
		encoding transferv1.BundleEncoding
		layout   transferv1.ZipLayout
	}

	tests := []struct {
		name     string
		input    input
		mockFunc func(ctrl *gomock.Controller) *transfer.Service
		errIs    error
		wantErr  string
		check    func(t *testing.T, payload []byte, ct, fname string)
	}{
		{
			name: "success JSON single bundle",
			input: input{
				encoding: transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.pdp.EXPECT().HasNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(true)
				m.namespaces.EXPECT().
					ListAll(gomock.Any()).
					Return([]*domain.Namespace{{Name: "ns1"}}, nil)
				m.configs.EXPECT().
					ListAllByNamespace(gomock.Any(), "ns1").
					Return([]*domain.Config{{Path: "/a.json", Namespace: "ns1"}}, nil)

				return svc
			},
			check: func(t *testing.T, payload []byte, ct, fname string) {
				t.Helper()

				assert.Equal(t, "application/json", ct)
				assert.Equal(t, "elara-export-all.json", fname)
				var bundle domain.AllBundle
				require.NoError(t, json.Unmarshal(payload, &bundle))
				require.Len(t, bundle.Namespaces, 1)
			},
		},
		{
			name: "success YAML single bundle",
			input: input{
				encoding: transferv1.BundleEncoding_BUNDLE_ENCODING_YAML,
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.pdp.EXPECT().HasNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(true)
				m.namespaces.EXPECT().
					ListAll(gomock.Any()).
					Return([]*domain.Namespace{{Name: "ns1"}}, nil)
				m.configs.EXPECT().
					ListAllByNamespace(gomock.Any(), "ns1").
					Return([]*domain.Config{{Path: "/a.yaml", Namespace: "ns1"}}, nil)

				return svc
			},
			check: func(t *testing.T, payload []byte, ct, fname string) {
				t.Helper()

				assert.Equal(t, "application/yaml", ct)
				assert.Equal(t, "elara-export-all.yaml", fname)
				var bundle domain.AllBundle
				require.NoError(t, yaml.Unmarshal(payload, &bundle))
				require.Len(t, bundle.Namespaces, 1)
			},
		},
		{
			name: "success JSON as zip",
			input: input{
				asZip:    true,
				encoding: transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.pdp.EXPECT().HasNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(true)
				m.namespaces.EXPECT().
					ListAll(gomock.Any()).
					Return([]*domain.Namespace{{Name: "ns1"}}, nil)
				m.configs.EXPECT().
					ListAllByNamespace(gomock.Any(), "ns1").
					Return([]*domain.Config{{Path: "/a.json", Namespace: "ns1"}}, nil)

				return svc
			},
			check: func(t *testing.T, payload []byte, ct, fname string) {
				t.Helper()

				assert.Equal(t, "application/zip", ct)
				assert.Equal(t, "elara-export-all.zip", fname)
				entries := readZipEntries(t, payload)
				assert.Contains(t, entries, "elara-export-all.json")
			},
		},
		{
			name: "success per-namespace ZIP",
			input: input{
				asZip:    true,
				encoding: transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
				layout:   transferv1.ZipLayout_ZIP_LAYOUT_PER_NAMESPACE,
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.pdp.EXPECT().
					HasNamespace(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(true).
					Times(2)
				m.namespaces.EXPECT().
					ListAll(gomock.Any()).
					Return([]*domain.Namespace{{Name: "ns1"}, {Name: "ns2"}}, nil)
				m.configs.EXPECT().
					ListAllByNamespace(gomock.Any(), "ns1").
					Return([]*domain.Config{{Path: "/a.json", Namespace: "ns1"}}, nil)
				m.configs.EXPECT().
					ListAllByNamespace(gomock.Any(), "ns2").
					Return([]*domain.Config{{Path: "/b.json", Namespace: "ns2"}}, nil)

				return svc
			},
			check: func(t *testing.T, payload []byte, ct, fname string) {
				t.Helper()

				assert.Equal(t, "application/zip", ct)
				entries := readZipEntries(t, payload)
				assert.Contains(t, entries, "namespaces/ns1.json")
				assert.Contains(t, entries, "namespaces/ns2.json")
				assert.Contains(t, entries, "index.json")
			},
		},
		{
			name: "empty namespaces",
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.namespaces.EXPECT().ListAll(gomock.Any()).Return([]*domain.Namespace{}, nil)

				return svc
			},
			check: func(t *testing.T, payload []byte, ct, fname string) {
				t.Helper()

				assert.Equal(t, "application/json", ct)
				var bundle domain.AllBundle
				require.NoError(t, json.Unmarshal(payload, &bundle))
				assert.Empty(t, bundle.Namespaces)
			},
		},
		{
			name: "storage error - list namespaces",
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.namespaces.EXPECT().ListAll(gomock.Any()).Return(nil, errors.New("db error"))

				return svc
			},
			wantErr: "list namespaces: db error",
		},
		{
			name: "storage error - list configs",
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.pdp.EXPECT().HasNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(true)
				m.namespaces.EXPECT().
					ListAll(gomock.Any()).
					Return([]*domain.Namespace{{Name: "ns1"}}, nil)
				m.configs.EXPECT().
					ListAllByNamespace(gomock.Any(), "ns1").
					Return(nil, errors.New("timeout"))

				return svc
			},
			wantErr: "list configs for namespace ns1: timeout",
		},
		{
			name: "partial access - skips unauthorized namespaces",
			input: input{
				encoding: transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.namespaces.EXPECT().
					ListAll(gomock.Any()).
					Return([]*domain.Namespace{{Name: "ns1"}, {Name: "ns2"}}, nil)
				m.pdp.EXPECT().
					HasNamespace("test@example.com", "ns1", domain.ActionRead).
					Return(true)
				m.pdp.EXPECT().
					HasNamespace("test@example.com", "ns2", domain.ActionRead).
					Return(false)
				m.configs.EXPECT().
					ListAllByNamespace(gomock.Any(), "ns1").
					Return([]*domain.Config{{Path: "/a.json", Namespace: "ns1"}}, nil)

				return svc
			},
			check: func(t *testing.T, payload []byte, ct, fname string) {
				t.Helper()

				var bundle domain.AllBundle
				require.NoError(t, json.Unmarshal(payload, &bundle))
				require.Len(t, bundle.Namespaces, 1)
				assert.Equal(t, "ns1", bundle.Namespaces[0].Namespace)
			},
		},
		{
			name: "all namespaces denied - returns empty bundle",
			input: input{
				encoding: transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.namespaces.EXPECT().
					ListAll(gomock.Any()).
					Return([]*domain.Namespace{{Name: "secret"}}, nil)
				m.pdp.EXPECT().
					HasNamespace("test@example.com", "secret", domain.ActionRead).
					Return(false)

				return svc
			},
			check: func(t *testing.T, payload []byte, ct, fname string) {
				t.Helper()

				var bundle domain.AllBundle
				require.NoError(t, json.Unmarshal(payload, &bundle))
				assert.Empty(t, bundle.Namespaces)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			payload, ct, fname, err := sut.ExportAll(
				transferTestCtx(t.Context()),
				tt.input.asZip,
				tt.input.encoding,
				tt.input.layout,
			)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)

			if tt.check != nil {
				tt.check(t, payload, ct, fname)
			}
		})
	}
}
