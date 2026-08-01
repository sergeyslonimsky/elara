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

func TestService_ExportNamespace(t *testing.T) {
	t.Parallel()

	type input struct {
		namespace string
		asZip     bool
		encoding  transferv1.BundleEncoding
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
			name: "success JSON",
			input: input{
				namespace: "my-ns",
				encoding:  transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.namespaces.EXPECT().
					Get(gomock.Any(), "my-ns").
					Return(&domain.Namespace{Name: "my-ns", Description: "desc"}, nil)
				m.configs.EXPECT().
					ListAllByNamespace(gomock.Any(), "my-ns").
					Return([]*domain.Config{{Path: "/c1", Namespace: "my-ns"}}, nil)

				return svc
			},
			check: func(t *testing.T, payload []byte, ct, fname string) {
				t.Helper()

				assert.Equal(t, "application/json", ct)
				assert.Equal(t, "my-ns-export.json", fname)
				var bundle domain.NamespaceBundle
				require.NoError(t, json.Unmarshal(payload, &bundle))
				assert.Equal(t, "my-ns", bundle.Namespace)
			},
		},
		{
			name: "success YAML",
			input: input{
				namespace: "my-ns",
				encoding:  transferv1.BundleEncoding_BUNDLE_ENCODING_YAML,
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.namespaces.EXPECT().
					Get(gomock.Any(), "my-ns").
					Return(&domain.Namespace{Name: "my-ns"}, nil)
				m.configs.EXPECT().
					ListAllByNamespace(gomock.Any(), "my-ns").
					Return([]*domain.Config{}, nil)

				return svc
			},
			check: func(t *testing.T, payload []byte, ct, fname string) {
				t.Helper()

				assert.Equal(t, "application/yaml", ct)
				assert.Equal(t, "my-ns-export.yaml", fname)
				var bundle domain.NamespaceBundle
				require.NoError(t, yaml.Unmarshal(payload, &bundle))
			},
		},
		{
			name: "namespace not found",
			input: input{
				namespace: "missing",
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.namespaces.EXPECT().Get(gomock.Any(), "missing").Return(nil, domain.ErrNotFound)

				return svc
			},
			errIs:   domain.ErrNotFound,
			wantErr: "get namespace",
		},
		{
			name: "storage error",
			input: input{
				namespace: "my-ns",
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.namespaces.EXPECT().
					Get(gomock.Any(), "my-ns").
					Return(&domain.Namespace{Name: "my-ns"}, nil)
				m.configs.EXPECT().
					ListAllByNamespace(gomock.Any(), "my-ns").
					Return(nil, errors.New("db error"))

				return svc
			},
			wantErr: "list configs: db error",
		},
		{
			name: "success as zip",
			input: input{
				namespace: "my-ns",
				asZip:     true,
				encoding:  transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.namespaces.EXPECT().
					Get(gomock.Any(), "my-ns").
					Return(&domain.Namespace{Name: "my-ns"}, nil)
				m.configs.EXPECT().
					ListAllByNamespace(gomock.Any(), "my-ns").
					Return([]*domain.Config{{Path: "/c1", Namespace: "my-ns"}}, nil)

				return svc
			},
			check: func(t *testing.T, payload []byte, ct, fname string) {
				t.Helper()

				assert.Equal(t, "application/zip", ct)
				assert.Equal(t, "my-ns-export.zip", fname)
				entries := readZipEntries(t, payload)
				assert.Contains(t, entries, "my-ns-export.json")
			},
		},
		{
			// sort.Slice's comparator only runs when Configs has 2+ entries.
			name: "multiple configs are sorted by path",
			input: input{
				namespace: "my-ns",
				encoding:  transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.namespaces.EXPECT().
					Get(gomock.Any(), "my-ns").
					Return(&domain.Namespace{Name: "my-ns"}, nil)
				m.configs.EXPECT().
					ListAllByNamespace(gomock.Any(), "my-ns").
					Return([]*domain.Config{
						{Path: "/z", Namespace: "my-ns"},
						{Path: "/a", Namespace: "my-ns"},
					}, nil)

				return svc
			},
			check: func(t *testing.T, payload []byte, ct, fname string) {
				t.Helper()

				var bundle domain.NamespaceBundle
				require.NoError(t, json.Unmarshal(payload, &bundle))
				require.Len(t, bundle.Configs, 2)
				assert.Equal(t, "/a", bundle.Configs[0].Path)
				assert.Equal(t, "/z", bundle.Configs[1].Path)
			},
		},
		{
			name: "lock state stripped",
			input: input{
				namespace: "locked-ns",
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.namespaces.EXPECT().
					Get(gomock.Any(), "locked-ns").
					Return(&domain.Namespace{Name: "locked-ns", Locked: true}, nil)
				m.configs.EXPECT().
					ListAllByNamespace(gomock.Any(), "locked-ns").
					Return([]*domain.Config{
						{
							Path:            "/locked.json",
							Namespace:       "locked-ns",
							Locked:          true,
							NamespaceLocked: true,
						},
					}, nil)

				return svc
			},
			check: func(t *testing.T, payload []byte, ct, fname string) {
				t.Helper()

				var raw map[string]any
				require.NoError(t, json.Unmarshal(payload, &raw))
				assert.NotContains(t, raw, "locked")
				configs, ok := raw["configs"].([]any)
				require.True(t, ok)
				entry, ok := configs[0].(map[string]any)
				require.True(t, ok)
				assert.NotContains(t, entry, "locked")
				assert.NotContains(t, entry, "namespaceLocked")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			payload, ct, fname, err := sut.ExportNamespace(
				transferTestCtx(t.Context()),
				tt.input.namespace,
				tt.input.asZip,
				tt.input.encoding,
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
