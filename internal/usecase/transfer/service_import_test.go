package transfer_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
	"github.com/sergeyslonimsky/elara/internal/usecase/transfer"
)

func TestService_Import(t *testing.T) {
	t.Parallel()

	type input struct {
		data            []byte
		resolution      transferv1.ConflictResolution
		dryRun          bool
		targetNamespace string
	}

	tests := []struct {
		name     string
		input    func(t *testing.T) input
		mockFunc func(ctrl *gomock.Controller) *transfer.Service
		errIs    error
		wantErr  string
		want     *domain.ImportReport
	}{
		{
			name: "success namespace bundle JSON new configs",
			input: func(t *testing.T) input {
				t.Helper()

				return input{
					data: func() []byte {
						b, err := json.Marshal(domain.NamespaceBundle{
							Namespace: "my-ns",
							Configs: []domain.BundleConfig{
								{Path: "/c1", Content: "{}", Format: domain.FormatJSON},
							},
							ExportedAt: time.Now(),
						})
						require.NoError(t, err)

						return b
					}(),
				}
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.pdp.EXPECT().HasNamespace(testUserID, "my-ns", domain.ActionWrite).Return(true)
				m.namespaces.EXPECT().Get(gomock.Any(), "my-ns").Return(nil, domain.ErrNotFound)
				m.namespaces.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
				m.configs.EXPECT().Get(gomock.Any(), "/c1", "my-ns").Return(nil, domain.ErrNotFound)
				m.configs.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

				return svc
			},
			want: &domain.ImportReport{Created: 1},
		},
		{
			name: "success all bundle JSON multiple namespaces",
			input: func(t *testing.T) input {
				t.Helper()

				return input{
					data: func() []byte {
						b, err := json.Marshal(domain.AllBundle{
							Namespaces: []domain.NamespaceBundle{
								{
									Namespace:  "ns1",
									Configs:    []domain.BundleConfig{{Path: "/a", Content: "{}"}},
									ExportedAt: time.Now(),
								},
								{
									Namespace:  "ns2",
									Configs:    []domain.BundleConfig{{Path: "/b", Content: "{}"}},
									ExportedAt: time.Now(),
								},
							},
						})
						require.NoError(t, err)

						return b
					}(),
				}
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.pdp.EXPECT().HasNamespace(testUserID, domain.DomainAll, domain.ActionWrite).Return(true)
				m.namespaces.EXPECT().
					Get(gomock.Any(), "ns1").
					Return(&domain.Namespace{Name: "ns1"}, nil)
				m.namespaces.EXPECT().
					Get(gomock.Any(), "ns2").
					Return(&domain.Namespace{Name: "ns2"}, nil)
				m.configs.EXPECT().Get(gomock.Any(), "/a", "ns1").Return(nil, domain.ErrNotFound)
				m.configs.EXPECT().Get(gomock.Any(), "/b", "ns2").Return(nil, domain.ErrNotFound)
				m.configs.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).Times(2)

				return svc
			},
			want: &domain.ImportReport{Created: 2},
		},
		{
			name: "conflict resolution skip",
			input: func(t *testing.T) input {
				t.Helper()

				return input{
					data: func() []byte {
						b, err := json.Marshal(domain.NamespaceBundle{
							Namespace:  "my-ns",
							Configs:    []domain.BundleConfig{{Path: "/c1", Content: "{}"}},
							ExportedAt: time.Now(),
						})
						require.NoError(t, err)

						return b
					}(),
					resolution: transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
				}
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.pdp.EXPECT().HasNamespace(testUserID, "my-ns", domain.ActionWrite).Return(true)
				m.namespaces.EXPECT().
					Get(gomock.Any(), "my-ns").
					Return(&domain.Namespace{Name: "my-ns"}, nil)
				m.configs.EXPECT().
					Get(gomock.Any(), "/c1", "my-ns").
					Return(&domain.Config{Path: "/c1", Namespace: "my-ns"}, nil)

				return svc
			},
			want: &domain.ImportReport{Skipped: 1},
		},
		{
			name: "conflict resolution overwrite",
			input: func(t *testing.T) input {
				t.Helper()

				return input{
					data: func() []byte {
						b, err := json.Marshal(domain.NamespaceBundle{
							Namespace: "my-ns",
							Configs:   []domain.BundleConfig{{Path: "/c1", Content: "{}"}},
						})
						require.NoError(t, err)

						return b
					}(),
					resolution: transferv1.ConflictResolution_CONFLICT_RESOLUTION_OVERWRITE,
				}
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.pdp.EXPECT().HasNamespace(testUserID, "my-ns", domain.ActionWrite).Return(true)
				m.namespaces.EXPECT().
					Get(gomock.Any(), "my-ns").
					Return(&domain.Namespace{Name: "my-ns"}, nil)
				m.configs.EXPECT().
					Get(gomock.Any(), "/c1", "my-ns").
					Return(&domain.Config{Path: "/c1", Namespace: "my-ns", Version: 42}, nil)
				m.configs.EXPECT().Update(gomock.Any(), gomock.Cond(func(x any) bool {
					cfg, _ := x.(*domain.Config)

					return cfg.Version == 42
				})).Return(nil)

				return svc
			},
			want: &domain.ImportReport{Updated: 1},
		},
		{
			name: "dry run",
			input: func(t *testing.T) input {
				t.Helper()

				return input{
					data: func() []byte {
						b, err := json.Marshal(domain.NamespaceBundle{
							Namespace: "my-ns",
							Configs:   []domain.BundleConfig{{Path: "/c1", Content: "{}"}},
						})
						require.NoError(t, err)

						return b
					}(),
					dryRun: true,
				}
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.pdp.EXPECT().HasNamespace(testUserID, "my-ns", domain.ActionWrite).Return(true)
				// Dry run only checks existence
				m.configs.EXPECT().Get(gomock.Any(), "/c1", "my-ns").Return(nil, domain.ErrNotFound)

				return svc
			},
			want: &domain.ImportReport{Created: 1, DryRun: true},
		},
		{
			name: "target namespace overrides",
			input: func(t *testing.T) input {
				t.Helper()

				return input{
					data: func() []byte {
						b, err := json.Marshal(domain.NamespaceBundle{
							Namespace: "original",
							Configs:   []domain.BundleConfig{{Path: "/c1", Content: "{}"}},
						})
						require.NoError(t, err)

						return b
					}(),
					targetNamespace: "target",
				}
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.namespaces.EXPECT().Get(gomock.Any(), "target").Return(nil, domain.ErrNotFound)
				m.namespaces.EXPECT().Create(gomock.Any(), gomock.Cond(func(x any) bool {
					ns, ok := x.(*domain.Namespace)
					if !ok {
						return false
					}

					return ns.Name == "target"
				})).Return(nil)
				m.configs.EXPECT().Get(
					gomock.Any(),
					"/c1",
					"target",
				).Return(nil, domain.ErrNotFound)
				m.configs.EXPECT().Create(gomock.Any(), gomock.Cond(func(x any) bool {
					cfg, ok := x.(*domain.Config)
					if !ok {
						return false
					}

					return cfg.Namespace == "target"
				})).Return(nil)

				return svc
			},
			want: &domain.ImportReport{Created: 1},
		},
		{
			name: "access denied - all bundle",
			input: func(t *testing.T) input {
				t.Helper()

				return input{
					data: func() []byte {
						b, err := json.Marshal(domain.AllBundle{
							Namespaces: []domain.NamespaceBundle{
								{
									Namespace: "ns1",
									Configs:   []domain.BundleConfig{{Path: "/a", Content: "{}"}},
								},
							},
						})
						require.NoError(t, err)

						return b
					}(),
				}
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.pdp.EXPECT().HasNamespace(testUserID, domain.DomainAll, domain.ActionWrite).Return(false)

				return svc
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "access denied - single namespace fallback",
			input: func(t *testing.T) input {
				t.Helper()

				return input{
					data: func() []byte {
						b, err := json.Marshal(domain.NamespaceBundle{
							Namespace: "my-ns",
							Configs:   []domain.BundleConfig{{Path: "/c1", Content: "{}"}},
						})
						require.NoError(t, err)

						return b
					}(),
				}
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, m := setupService(t, ctrl)
				m.pdp.EXPECT().HasNamespace(testUserID, "my-ns", domain.ActionWrite).Return(false)

				return svc
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "validation error - empty namespace",
			input: func(t *testing.T) input {
				t.Helper()

				return input{
					data: func() []byte {
						b, err := json.Marshal(domain.NamespaceBundle{
							Namespace: "",
							Configs:   []domain.BundleConfig{},
						})
						require.NoError(t, err)

						return b
					}(),
				}
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, _ := setupService(t, ctrl)

				return svc
			},
			wantErr: "validation: namespace: bundle namespace is required",
		},
		{
			name: "validation error - corrupt data",
			input: func(t *testing.T) input {
				t.Helper()

				return input{
					data: []byte(`{corrupt`),
				}
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Service {
				svc, _ := setupService(t, ctrl)

				return svc
			},
			wantErr: "validation: data: parse bundle: json unmarshal bundle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)
			testInput := tt.input(t)

			got, err := sut.Import(
				transferTestCtx(t.Context()),
				testInput.data,
				testInput.resolution,
				testInput.dryRun,
				testInput.targetNamespace,
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
			assert.Equal(t, tt.want.Created, got.Created)
			assert.Equal(t, tt.want.Updated, got.Updated)
			assert.Equal(t, tt.want.Skipped, got.Skipped)
			assert.Equal(t, tt.want.Failed, got.Failed)
			assert.Equal(t, tt.want.DryRun, got.DryRun)
		})
	}
}
