package transfer

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/sergeyslonimsky/elara/internal/domain"
	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
)

// ---------------------------------------------------------------------------
// isZIP
// ---------------------------------------------------------------------------

func TestIsZIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "valid ZIP magic bytes",
			data:     []byte{0x50, 0x4B, 0x03, 0x04, 0x00},
			expected: true,
		},
		{
			name:     "JSON data is not ZIP",
			data:     []byte(`{"key":"value"}`),
			expected: false,
		},
		{
			name:     "YAML data is not ZIP",
			data:     []byte("key: value\n"),
			expected: false,
		},
		{
			name:     "empty data",
			data:     []byte{},
			expected: false,
		},
		{
			name:     "too short",
			data:     []byte{0x50, 0x4B},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, isZIP(tc.data))
		})
	}
}

// ---------------------------------------------------------------------------
// isYAML
// ---------------------------------------------------------------------------

func TestIsYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "JSON object",
			data:     []byte(`{"key":"value"}`),
			expected: false,
		},
		{
			name:     "JSON array",
			data:     []byte(`[{"key":"value"}]`),
			expected: false,
		},
		{
			name:     "YAML document",
			data:     []byte("key: value\n"),
			expected: true,
		},
		{
			name:     "YAML with leading whitespace",
			data:     []byte("   \nkey: value\n"),
			expected: true,
		},
		{
			name:     "empty data",
			data:     []byte{},
			expected: false,
		},
		{
			name:     "whitespace only",
			data:     []byte("   \n  \t "),
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, isYAML(tc.data))
		})
	}
}

// ---------------------------------------------------------------------------
// marshalBundle
// ---------------------------------------------------------------------------

func TestMarshalBundle(t *testing.T) {
	t.Parallel()

	bundle := domain.NamespaceBundle{
		Namespace:   "test-ns",
		Description: "test description",
		ExportedAt:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Configs:     []domain.BundleConfig{},
	}

	tests := []struct {
		name           string
		enc            transferv1.BundleEncoding
		expectedCT     string
		checkUnmarshal func(t *testing.T, data []byte)
	}{
		{
			name:       "JSON encoding (default)",
			enc:        transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
			expectedCT: contentTypeJSON,
			checkUnmarshal: func(t *testing.T, data []byte) {
				t.Helper()
				var result domain.NamespaceBundle
				require.NoError(t, json.Unmarshal(data, &result))
				assert.Equal(t, bundle.Namespace, result.Namespace)
			},
		},
		{
			name:       "YAML encoding",
			enc:        transferv1.BundleEncoding_BUNDLE_ENCODING_YAML,
			expectedCT: contentTypeYAML,
			checkUnmarshal: func(t *testing.T, data []byte) {
				t.Helper()
				var result domain.NamespaceBundle
				require.NoError(t, yaml.Unmarshal(data, &result))
				assert.Equal(t, bundle.Namespace, result.Namespace)
			},
		},
		{
			name:       "unspecified encoding defaults to JSON",
			enc:        transferv1.BundleEncoding_BUNDLE_ENCODING_UNSPECIFIED,
			expectedCT: contentTypeJSON,
			checkUnmarshal: func(t *testing.T, data []byte) {
				t.Helper()
				var result domain.NamespaceBundle
				require.NoError(t, json.Unmarshal(data, &result))
				assert.Equal(t, bundle.Namespace, result.Namespace)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			data, ct, err := marshalBundle(bundle, tc.enc)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedCT, ct)
			assert.NotEmpty(t, data)
			tc.checkUnmarshal(t, data)
		})
	}
}

// ---------------------------------------------------------------------------
// wrapInZip / unzipIfNeeded roundtrip
// ---------------------------------------------------------------------------

func TestWrapInZip_UnzipRoundtrip(t *testing.T) {
	t.Parallel()

	original := []byte(`{"namespace":"test"}`)

	zipped, err := wrapInZip("bundle.json", original)
	require.NoError(t, err)
	assert.True(t, isZIP(zipped))

	unzipped, err := unzipIfNeeded(zipped)
	require.NoError(t, err)
	assert.Equal(t, original, unzipped)
}

func TestUnzipIfNeeded_NonZIPPassthrough(t *testing.T) {
	t.Parallel()

	data := []byte(`{"namespace":"test"}`)

	result, err := unzipIfNeeded(data)
	require.NoError(t, err)
	assert.Equal(t, data, result)
}

func TestUnzipIfNeeded_EmptyZip(t *testing.T) {
	t.Parallel()

	// Build a ZIP with only a directory entry (no files).
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	_, err := zw.Create("dir/")
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	_, err = unzipIfNeeded(buf.Bytes())
	require.ErrorIs(t, err, errEmptyZip)
}

// ---------------------------------------------------------------------------
// unmarshalNamespaceBundle
// ---------------------------------------------------------------------------

func TestUnmarshalNamespaceBundle(t *testing.T) {
	t.Parallel()

	bundle := domain.NamespaceBundle{
		Namespace:   "my-ns",
		Description: "my description",
		ExportedAt:  time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
		Configs: []domain.BundleConfig{
			{Path: "/config.json", Content: `{"key":"value"}`, Format: domain.FormatJSON},
		},
	}

	jsonData, err := json.Marshal(bundle)
	require.NoError(t, err)

	yamlData, err := yaml.Marshal(bundle)
	require.NoError(t, err)

	zipData, err := wrapInZip("bundle.json", jsonData)
	require.NoError(t, err)

	tests := []struct {
		name string
		data []byte
	}{
		{name: "JSON", data: jsonData},
		{name: "YAML", data: yamlData},
		{name: "ZIP containing JSON", data: zipData},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := unmarshalNamespaceBundle(tc.data)
			require.NoError(t, err)
			assert.Equal(t, bundle.Namespace, result.Namespace)
			assert.Equal(t, bundle.Description, result.Description)
			require.Len(t, result.Configs, 1)
			assert.Equal(t, bundle.Configs[0].Path, result.Configs[0].Path)
		})
	}
}

func TestUnmarshalNamespaceBundle_InvalidData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "corrupt JSON",
			data: []byte(`{corrupt`),
		},
		{
			name: "corrupt YAML",
			data: []byte("key: :\n  bad yaml: [\n"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := unmarshalNamespaceBundle(tc.data)
			require.Error(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// unmarshalAllBundle
// ---------------------------------------------------------------------------

func TestUnmarshalAllBundle(t *testing.T) {
	t.Parallel()

	bundle := domain.AllBundle{
		ExportedAt: time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
		Namespaces: []domain.NamespaceBundle{
			{
				Namespace:   "ns1",
				Description: "first namespace",
				ExportedAt:  time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
				Configs:     []domain.BundleConfig{},
			},
		},
	}

	jsonData, err := json.Marshal(bundle)
	require.NoError(t, err)

	yamlData, err := yaml.Marshal(bundle)
	require.NoError(t, err)

	zipData, err := wrapInZip("all-bundle.json", jsonData)
	require.NoError(t, err)

	tests := []struct {
		name string
		data []byte
	}{
		{name: "JSON", data: jsonData},
		{name: "YAML", data: yamlData},
		{name: "ZIP containing JSON", data: zipData},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := unmarshalAllBundle(tc.data)
			require.NoError(t, err)
			require.Len(t, result.Namespaces, 1)
			assert.Equal(t, "ns1", result.Namespaces[0].Namespace)
		})
	}
}

// ---------------------------------------------------------------------------
// bundleExtension
// ---------------------------------------------------------------------------

func TestBundleExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ct       string
		asZip    bool
		expected string
	}{
		{name: "JSON not zip", ct: contentTypeJSON, asZip: false, expected: ".json"},
		{name: "YAML not zip", ct: contentTypeYAML, asZip: false, expected: ".yaml"},
		{name: "JSON as zip", ct: contentTypeJSON, asZip: true, expected: ".zip"},
		{name: "YAML as zip", ct: contentTypeYAML, asZip: true, expected: ".zip"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, bundleExtension(tc.ct, tc.asZip))
		})
	}
}
