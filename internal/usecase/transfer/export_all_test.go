package transfer_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gopkg.in/yaml.v3"

	"github.com/sergeyslonimsky/elara/internal/domain"
	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
	"github.com/sergeyslonimsky/elara/internal/usecase/transfer"
	transfer_mock "github.com/sergeyslonimsky/elara/internal/usecase/transfer/mocks"
)

// readZipEntries reads a ZIP archive and returns a map of filename -> content.
func readZipEntries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)

	entries := make(map[string][]byte, len(zr.File))

	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)

		content, err := io.ReadAll(rc)

		require.NoError(t, rc.Close())
		require.NoError(t, err)

		entries[f.Name] = content
	}

	return entries
}

// ---------------------------------------------------------------------------
// Tests: happy path — single flat bundle
// ---------------------------------------------------------------------------

func TestExportAllUseCase_JSONEncoding_SingleBundle(t *testing.T) {
	t.Parallel()

	namespaces := []*domain.Namespace{
		{Name: "ns1", Description: "first"},
	}
	configsByNS := map[string][]*domain.Config{
		"ns1": {
			{Path: "/a.json", Content: `{}`, Format: domain.FormatJSON, Namespace: "ns1"},
		},
	}

	ctrl := gomock.NewController(t)
	lister := transfer_mock.NewMockexportAllConfigLister(ctrl)
	nsLister := transfer_mock.NewMockexportAllNSLister(ctrl)

	nsLister.EXPECT().List(gomock.Any()).Return(namespaces, nil)
	lister.EXPECT().ListAllByNamespace(gomock.Any(), "ns1").Return(configsByNS["ns1"], nil)

	uc := transfer.NewExportAllUseCase(lister, nsLister)

	payload, ct, fname, err := uc.Execute(
		t.Context(),
		false,
		transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
		transferv1.ZipLayout_ZIP_LAYOUT_UNSPECIFIED,
	)

	require.NoError(t, err)
	assert.Equal(t, "application/json", ct)
	assert.Equal(t, "elara-export-all.json", fname)

	var bundle domain.AllBundle
	require.NoError(t, json.Unmarshal(payload, &bundle))
	require.Len(t, bundle.Namespaces, 1)
	assert.Equal(t, "ns1", bundle.Namespaces[0].Namespace)
	require.Len(t, bundle.Namespaces[0].Configs, 1)
}

func TestExportAllUseCase_YAMLEncoding_SingleBundle(t *testing.T) {
	t.Parallel()

	namespaces := []*domain.Namespace{
		{Name: "yaml-ns"},
	}
	configsByNS := map[string][]*domain.Config{
		"yaml-ns": {
			{Path: "/b.yaml", Content: "key: value", Format: domain.FormatYAML, Namespace: "yaml-ns"},
		},
	}

	ctrl := gomock.NewController(t)
	lister := transfer_mock.NewMockexportAllConfigLister(ctrl)
	nsLister := transfer_mock.NewMockexportAllNSLister(ctrl)

	nsLister.EXPECT().List(gomock.Any()).Return(namespaces, nil)
	lister.EXPECT().ListAllByNamespace(gomock.Any(), "yaml-ns").Return(configsByNS["yaml-ns"], nil)

	uc := transfer.NewExportAllUseCase(lister, nsLister)

	payload, ct, fname, err := uc.Execute(
		t.Context(),
		false,
		transferv1.BundleEncoding_BUNDLE_ENCODING_YAML,
		transferv1.ZipLayout_ZIP_LAYOUT_UNSPECIFIED,
	)

	require.NoError(t, err)
	assert.Equal(t, "application/yaml", ct)
	assert.Equal(t, "elara-export-all.yaml", fname)

	var bundle domain.AllBundle
	require.NoError(t, yaml.Unmarshal(payload, &bundle))
	require.Len(t, bundle.Namespaces, 1)
	assert.Equal(t, "yaml-ns", bundle.Namespaces[0].Namespace)
}

func TestExportAllUseCase_UnspecifiedEncoding_DefaultsToJSON(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	lister := transfer_mock.NewMockexportAllConfigLister(ctrl)
	nsLister := transfer_mock.NewMockexportAllNSLister(ctrl)

	nsLister.EXPECT().List(gomock.Any()).Return([]*domain.Namespace{}, nil)

	uc := transfer.NewExportAllUseCase(lister, nsLister)

	_, ct, fname, err := uc.Execute(
		t.Context(),
		false,
		transferv1.BundleEncoding_BUNDLE_ENCODING_UNSPECIFIED,
		transferv1.ZipLayout_ZIP_LAYOUT_UNSPECIFIED,
	)

	require.NoError(t, err)
	assert.Equal(t, "application/json", ct)
	assert.Equal(t, "elara-export-all.json", fname)
}

// ---------------------------------------------------------------------------
// Tests: asZip=true (flat ZIP, default layout)
// ---------------------------------------------------------------------------

func TestExportAllUseCase_AsZip_JSONEncoding(t *testing.T) {
	t.Parallel()

	namespaces := []*domain.Namespace{{Name: "ns1"}}
	configsByNS := map[string][]*domain.Config{
		"ns1": {{Path: "/c.json", Content: `{}`, Format: domain.FormatJSON, Namespace: "ns1"}},
	}

	ctrl := gomock.NewController(t)
	lister := transfer_mock.NewMockexportAllConfigLister(ctrl)
	nsLister := transfer_mock.NewMockexportAllNSLister(ctrl)

	nsLister.EXPECT().List(gomock.Any()).Return(namespaces, nil)
	lister.EXPECT().ListAllByNamespace(gomock.Any(), "ns1").Return(configsByNS["ns1"], nil)

	uc := transfer.NewExportAllUseCase(lister, nsLister)

	payload, ct, fname, err := uc.Execute(
		t.Context(),
		true,
		transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
		transferv1.ZipLayout_ZIP_LAYOUT_UNSPECIFIED,
	)

	require.NoError(t, err)
	assert.Equal(t, "application/zip", ct)
	assert.Equal(t, "elara-export-all.zip", fname)

	// Verify ZIP magic bytes.
	require.GreaterOrEqual(t, len(payload), 4)
	assert.Equal(t, byte(0x50), payload[0])
	assert.Equal(t, byte(0x4B), payload[1])

	// The ZIP should contain exactly one entry: elara-export-all.json
	entries := readZipEntries(t, payload)
	require.Contains(t, entries, "elara-export-all.json")

	var bundle domain.AllBundle
	require.NoError(t, json.Unmarshal(entries["elara-export-all.json"], &bundle))
	require.Len(t, bundle.Namespaces, 1)
}

func TestExportAllUseCase_AsZip_YAMLEncoding(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	lister := transfer_mock.NewMockexportAllConfigLister(ctrl)
	nsLister := transfer_mock.NewMockexportAllNSLister(ctrl)

	nsLister.EXPECT().List(gomock.Any()).Return([]*domain.Namespace{}, nil)

	uc := transfer.NewExportAllUseCase(lister, nsLister)

	payload, ct, fname, err := uc.Execute(
		t.Context(),
		true,
		transferv1.BundleEncoding_BUNDLE_ENCODING_YAML,
		transferv1.ZipLayout_ZIP_LAYOUT_UNSPECIFIED,
	)

	require.NoError(t, err)
	assert.Equal(t, "application/zip", ct)
	assert.Equal(t, "elara-export-all.zip", fname)

	entries := readZipEntries(t, payload)
	require.Contains(t, entries, "elara-export-all.yaml")
}

// ---------------------------------------------------------------------------
// Tests: ZipLayout_ZIP_LAYOUT_PER_NAMESPACE
// ---------------------------------------------------------------------------

func TestExportAllUseCase_PerNamespaceZip_JSON(t *testing.T) {
	t.Parallel()

	namespaces := []*domain.Namespace{
		{Name: "ns1", Description: "first"},
		{Name: "ns2", Description: "second"},
	}
	configsByNS := map[string][]*domain.Config{
		"ns1": {{Path: "/a.json", Content: `{}`, Format: domain.FormatJSON, Namespace: "ns1"}},
		"ns2": {
			{Path: "/b.json", Content: `{}`, Format: domain.FormatJSON, Namespace: "ns2"},
			{Path: "/c.yaml", Content: "k: v", Format: domain.FormatYAML, Namespace: "ns2"},
		},
	}

	ctrl := gomock.NewController(t)
	lister := transfer_mock.NewMockexportAllConfigLister(ctrl)
	nsLister := transfer_mock.NewMockexportAllNSLister(ctrl)

	nsLister.EXPECT().List(gomock.Any()).Return(namespaces, nil)
	lister.EXPECT().ListAllByNamespace(gomock.Any(), "ns1").Return(configsByNS["ns1"], nil)
	lister.EXPECT().ListAllByNamespace(gomock.Any(), "ns2").Return(configsByNS["ns2"], nil)

	uc := transfer.NewExportAllUseCase(lister, nsLister)

	payload, ct, fname, err := uc.Execute(
		t.Context(),
		true,
		transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
		transferv1.ZipLayout_ZIP_LAYOUT_PER_NAMESPACE,
	)

	require.NoError(t, err)
	assert.Equal(t, "application/zip", ct)
	assert.Equal(t, "elara-export-all.zip", fname)

	entries := readZipEntries(t, payload)

	// Expect per-namespace files plus an index.
	assert.Contains(t, entries, "namespaces/ns1.json")
	assert.Contains(t, entries, "namespaces/ns2.json")
	assert.Contains(t, entries, "index.json")

	// Verify ns1 content.
	var ns1Bundle domain.NamespaceBundle
	require.NoError(t, json.Unmarshal(entries["namespaces/ns1.json"], &ns1Bundle))
	assert.Equal(t, "ns1", ns1Bundle.Namespace)
	require.Len(t, ns1Bundle.Configs, 1)

	// Verify ns2 content.
	var ns2Bundle domain.NamespaceBundle
	require.NoError(t, json.Unmarshal(entries["namespaces/ns2.json"], &ns2Bundle))
	assert.Equal(t, "ns2", ns2Bundle.Namespace)
	require.Len(t, ns2Bundle.Configs, 2)

	// Verify index lists both namespaces.
	var idx struct {
		Namespaces []string `json:"namespaces"`
	}
	require.NoError(t, json.Unmarshal(entries["index.json"], &idx))
	assert.ElementsMatch(t, []string{"ns1", "ns2"}, idx.Namespaces)
}

func TestExportAllUseCase_PerNamespaceZip_YAML(t *testing.T) {
	t.Parallel()

	namespaces := []*domain.Namespace{{Name: "ns1"}}
	configsByNS := map[string][]*domain.Config{
		"ns1": {{Path: "/a.json", Content: `{}`, Format: domain.FormatJSON, Namespace: "ns1"}},
	}

	ctrl := gomock.NewController(t)
	lister := transfer_mock.NewMockexportAllConfigLister(ctrl)
	nsLister := transfer_mock.NewMockexportAllNSLister(ctrl)

	nsLister.EXPECT().List(gomock.Any()).Return(namespaces, nil)
	lister.EXPECT().ListAllByNamespace(gomock.Any(), "ns1").Return(configsByNS["ns1"], nil)

	uc := transfer.NewExportAllUseCase(lister, nsLister)

	payload, ct, _, err := uc.Execute(
		t.Context(),
		true,
		transferv1.BundleEncoding_BUNDLE_ENCODING_YAML,
		transferv1.ZipLayout_ZIP_LAYOUT_PER_NAMESPACE,
	)

	require.NoError(t, err)
	assert.Equal(t, "application/zip", ct)

	entries := readZipEntries(t, payload)
	assert.Contains(t, entries, "namespaces/ns1.yaml")
	assert.Contains(t, entries, "index.yaml")

	var ns1Bundle domain.NamespaceBundle
	require.NoError(t, yaml.Unmarshal(entries["namespaces/ns1.yaml"], &ns1Bundle))
	assert.Equal(t, "ns1", ns1Bundle.Namespace)
}

// ---------------------------------------------------------------------------
// Tests: empty namespaces list
// ---------------------------------------------------------------------------

func TestExportAllUseCase_EmptyNamespaces(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	lister := transfer_mock.NewMockexportAllConfigLister(ctrl)
	nsLister := transfer_mock.NewMockexportAllNSLister(ctrl)

	nsLister.EXPECT().List(gomock.Any()).Return([]*domain.Namespace{}, nil)

	uc := transfer.NewExportAllUseCase(lister, nsLister)

	payload, ct, fname, err := uc.Execute(
		t.Context(),
		false,
		transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
		transferv1.ZipLayout_ZIP_LAYOUT_UNSPECIFIED,
	)

	require.NoError(t, err)
	assert.Equal(t, "application/json", ct)
	assert.Equal(t, "elara-export-all.json", fname)

	var bundle domain.AllBundle
	require.NoError(t, json.Unmarshal(payload, &bundle))
	assert.Empty(t, bundle.Namespaces)
}

// ---------------------------------------------------------------------------
// Tests: error propagation
// ---------------------------------------------------------------------------

func TestExportAllUseCase_NSListerError_Propagated(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	lister := transfer_mock.NewMockexportAllConfigLister(ctrl)
	nsLister := transfer_mock.NewMockexportAllNSLister(ctrl)

	nsLister.EXPECT().List(gomock.Any()).Return(nil, errors.New("storage unavailable"))

	uc := transfer.NewExportAllUseCase(lister, nsLister)

	_, _, _, err := uc.Execute(
		t.Context(),
		false,
		transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
		transferv1.ZipLayout_ZIP_LAYOUT_UNSPECIFIED,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "list namespaces")
	assert.Contains(t, err.Error(), "storage unavailable")
}

func TestExportAllUseCase_ConfigListerError_Propagated(t *testing.T) {
	t.Parallel()

	namespaces := []*domain.Namespace{{Name: "failing-ns"}}

	ctrl := gomock.NewController(t)
	lister := transfer_mock.NewMockexportAllConfigLister(ctrl)
	nsLister := transfer_mock.NewMockexportAllNSLister(ctrl)

	nsLister.EXPECT().List(gomock.Any()).Return(namespaces, nil)
	lister.EXPECT().ListAllByNamespace(gomock.Any(), "failing-ns").Return(nil, errors.New("timeout"))

	uc := transfer.NewExportAllUseCase(lister, nsLister)

	_, _, _, err := uc.Execute(
		t.Context(),
		false,
		transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
		transferv1.ZipLayout_ZIP_LAYOUT_UNSPECIFIED,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "list configs for namespace failing-ns")
	assert.Contains(t, err.Error(), "timeout")
}

// ---------------------------------------------------------------------------
// Tests: metadata preserved across all namespaces
// ---------------------------------------------------------------------------

func TestExportAllUseCase_ConfigMetadata_Preserved(t *testing.T) {
	t.Parallel()

	namespaces := []*domain.Namespace{{Name: "meta-ns"}}
	configsByNS := map[string][]*domain.Config{
		"meta-ns": {
			{
				Path:      "/config.json",
				Content:   `{}`,
				Format:    domain.FormatJSON,
				Namespace: "meta-ns",
				Metadata:  map[string]string{"region": "us-east-1"},
			},
		},
	}

	ctrl := gomock.NewController(t)
	lister := transfer_mock.NewMockexportAllConfigLister(ctrl)
	nsLister := transfer_mock.NewMockexportAllNSLister(ctrl)

	nsLister.EXPECT().List(gomock.Any()).Return(namespaces, nil)
	lister.EXPECT().ListAllByNamespace(gomock.Any(), "meta-ns").Return(configsByNS["meta-ns"], nil)

	uc := transfer.NewExportAllUseCase(lister, nsLister)

	payload, _, _, err := uc.Execute(
		t.Context(),
		false,
		transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
		transferv1.ZipLayout_ZIP_LAYOUT_UNSPECIFIED,
	)

	require.NoError(t, err)

	var bundle domain.AllBundle
	require.NoError(t, json.Unmarshal(payload, &bundle))
	require.Len(t, bundle.Namespaces, 1)
	require.Len(t, bundle.Namespaces[0].Configs, 1)
	assert.Equal(t, map[string]string{"region": "us-east-1"}, bundle.Namespaces[0].Configs[0].Metadata)
}
