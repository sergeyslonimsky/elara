package transfer_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/sergeyslonimsky/elara/internal/domain"
	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
	"github.com/sergeyslonimsky/elara/internal/usecase/transfer"
)

// ---------------------------------------------------------------------------
// Mocks for ExportNamespaceUseCase
// ---------------------------------------------------------------------------

type mockExportNSConfigLister struct {
	configs []*domain.Config
	err     error
}

func (m *mockExportNSConfigLister) ListAllByNamespace(_ context.Context, _ string) ([]*domain.Config, error) {
	return m.configs, m.err
}

type mockExportNSChecker struct {
	namespace *domain.Namespace
	err       error
}

func (m *mockExportNSChecker) Get(_ context.Context, _ string) (*domain.Namespace, error) {
	return m.namespace, m.err
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newExportNSUC(
	lister *mockExportNSConfigLister,
	checker *mockExportNSChecker,
) *transfer.ExportNamespaceUseCase {
	return transfer.NewExportNamespaceUseCase(lister, checker)
}

// ---------------------------------------------------------------------------
// Tests: happy path
// ---------------------------------------------------------------------------

func TestExportNamespaceUseCase_JSONEncoding(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	ns := &domain.Namespace{Name: "my-ns", Description: "my description"}
	configs := []*domain.Config{
		{Path: "/config.json", Content: `{"key":"value"}`, Format: domain.FormatJSON, Namespace: "my-ns"},
	}

	uc := newExportNSUC(
		&mockExportNSConfigLister{configs: configs},
		&mockExportNSChecker{namespace: ns},
	)

	payload, ct, fname, err := uc.Execute(ctx, "my-ns", false, transferv1.BundleEncoding_BUNDLE_ENCODING_JSON)

	require.NoError(t, err)
	assert.Equal(t, "application/json", ct)
	assert.Equal(t, "my-ns-export.json", fname)

	var bundle domain.NamespaceBundle
	require.NoError(t, json.Unmarshal(payload, &bundle))
	assert.Equal(t, "my-ns", bundle.Namespace)
	assert.Equal(t, "my description", bundle.Description)
	require.Len(t, bundle.Configs, 1)
	assert.Equal(t, "/config.json", bundle.Configs[0].Path)
}

func TestExportNamespaceUseCase_YAMLEncoding(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	ns := &domain.Namespace{Name: "yaml-ns", Description: "yaml ns description"}
	configs := []*domain.Config{
		{Path: "/config.yaml", Content: "key: value", Format: domain.FormatYAML, Namespace: "yaml-ns"},
	}

	uc := newExportNSUC(
		&mockExportNSConfigLister{configs: configs},
		&mockExportNSChecker{namespace: ns},
	)

	payload, ct, fname, err := uc.Execute(ctx, "yaml-ns", false, transferv1.BundleEncoding_BUNDLE_ENCODING_YAML)

	require.NoError(t, err)
	assert.Equal(t, "application/yaml", ct)
	assert.Equal(t, "yaml-ns-export.yaml", fname)

	var bundle domain.NamespaceBundle
	require.NoError(t, yaml.Unmarshal(payload, &bundle))
	assert.Equal(t, "yaml-ns", bundle.Namespace)
}

func TestExportNamespaceUseCase_UnspecifiedEncoding_DefaultsToJSON(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	ns := &domain.Namespace{Name: "my-ns"}
	uc := newExportNSUC(
		&mockExportNSConfigLister{configs: []*domain.Config{}},
		&mockExportNSChecker{namespace: ns},
	)

	payload, ct, fname, err := uc.Execute(ctx, "my-ns", false, transferv1.BundleEncoding_BUNDLE_ENCODING_UNSPECIFIED)

	require.NoError(t, err)
	assert.Equal(t, "application/json", ct)
	assert.Equal(t, "my-ns-export.json", fname)

	var bundle domain.NamespaceBundle
	require.NoError(t, json.Unmarshal(payload, &bundle))
	assert.Equal(t, "my-ns", bundle.Namespace)
}

// ---------------------------------------------------------------------------
// Tests: asZip=true
// ---------------------------------------------------------------------------

func TestExportNamespaceUseCase_AsZip_JSON(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	ns := &domain.Namespace{Name: "zip-ns"}
	uc := newExportNSUC(
		&mockExportNSConfigLister{configs: []*domain.Config{}},
		&mockExportNSChecker{namespace: ns},
	)

	payload, ct, fname, err := uc.Execute(ctx, "zip-ns", true, transferv1.BundleEncoding_BUNDLE_ENCODING_JSON)

	require.NoError(t, err)
	assert.Equal(t, "application/zip", ct)
	assert.Equal(t, "zip-ns-export.zip", fname)
	// Verify the ZIP magic bytes.
	require.GreaterOrEqual(t, len(payload), 4)
	assert.Equal(t, byte(0x50), payload[0])
	assert.Equal(t, byte(0x4B), payload[1])
}

func TestExportNamespaceUseCase_AsZip_YAML(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	ns := &domain.Namespace{Name: "zip-yaml-ns"}
	uc := newExportNSUC(
		&mockExportNSConfigLister{configs: []*domain.Config{}},
		&mockExportNSChecker{namespace: ns},
	)

	payload, ct, fname, err := uc.Execute(ctx, "zip-yaml-ns", true, transferv1.BundleEncoding_BUNDLE_ENCODING_YAML)

	require.NoError(t, err)
	assert.Equal(t, "application/zip", ct)
	assert.Equal(t, "zip-yaml-ns-export.zip", fname)
	require.GreaterOrEqual(t, len(payload), 4)
	assert.Equal(t, byte(0x50), payload[0])
}

// ---------------------------------------------------------------------------
// Tests: empty config list
// ---------------------------------------------------------------------------

func TestExportNamespaceUseCase_EmptyConfigs(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	ns := &domain.Namespace{Name: "empty-ns", Description: "no configs here"}
	uc := newExportNSUC(
		&mockExportNSConfigLister{configs: []*domain.Config{}},
		&mockExportNSChecker{namespace: ns},
	)

	payload, ct, fname, err := uc.Execute(ctx, "empty-ns", false, transferv1.BundleEncoding_BUNDLE_ENCODING_JSON)

	require.NoError(t, err)
	assert.Equal(t, "application/json", ct)
	assert.Equal(t, "empty-ns-export.json", fname)

	var bundle domain.NamespaceBundle
	require.NoError(t, json.Unmarshal(payload, &bundle))
	assert.Equal(t, "empty-ns", bundle.Namespace)
	assert.Empty(t, bundle.Configs)
}

// ---------------------------------------------------------------------------
// Tests: error propagation
// ---------------------------------------------------------------------------

func TestExportNamespaceUseCase_NamespaceNotFound_Error(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	uc := newExportNSUC(
		&mockExportNSConfigLister{},
		&mockExportNSChecker{err: domain.ErrNotFound},
	)

	_, _, _, err := uc.Execute(
		ctx,
		"missing-ns",
		false,
		transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get namespace")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestExportNamespaceUseCase_ConfigListerError_Propagated(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	ns := &domain.Namespace{Name: "my-ns"}
	uc := newExportNSUC(
		&mockExportNSConfigLister{err: errors.New("db connection lost")},
		&mockExportNSChecker{namespace: ns},
	)

	_, _, _, err := uc.Execute(
		ctx,
		"my-ns",
		false,
		transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "list configs")
	assert.Contains(t, err.Error(), "db connection lost")
}

// ---------------------------------------------------------------------------
// Tests: metadata is preserved
// ---------------------------------------------------------------------------

func TestExportNamespaceUseCase_ConfigMetadata_Preserved(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	ns := &domain.Namespace{Name: "meta-ns"}
	configs := []*domain.Config{
		{
			Path:      "/config.json",
			Content:   `{}`,
			Format:    domain.FormatJSON,
			Namespace: "meta-ns",
			Metadata:  map[string]string{"env": "prod", "owner": "team-a"},
		},
	}

	uc := newExportNSUC(
		&mockExportNSConfigLister{configs: configs},
		&mockExportNSChecker{namespace: ns},
	)

	payload, _, _, err := uc.Execute(ctx, "meta-ns", false, transferv1.BundleEncoding_BUNDLE_ENCODING_JSON)

	require.NoError(t, err)

	var bundle domain.NamespaceBundle
	require.NoError(t, json.Unmarshal(payload, &bundle))
	require.Len(t, bundle.Configs, 1)
	assert.Equal(t, map[string]string{"env": "prod", "owner": "team-a"}, bundle.Configs[0].Metadata)
}

// Lock state is per-instance, not part of the bundle. Exports must never leak
// it, so an import produces fresh, unlocked configs.
func TestExportNamespaceUseCase_LockState_Stripped(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	ns := &domain.Namespace{Name: "locked-ns", Locked: true}
	configs := []*domain.Config{
		{
			Path:            "/locked.json",
			Content:         `{}`,
			Format:          domain.FormatJSON,
			Namespace:       "locked-ns",
			Locked:          true,
			NamespaceLocked: true,
		},
		{
			Path:            "/unlocked.json",
			Content:         `{}`,
			Format:          domain.FormatJSON,
			Namespace:       "locked-ns",
			Locked:          false,
			NamespaceLocked: true,
		},
	}

	uc := newExportNSUC(
		&mockExportNSConfigLister{configs: configs},
		&mockExportNSChecker{namespace: ns},
	)

	payload, _, _, err := uc.Execute(ctx, "locked-ns", false, transferv1.BundleEncoding_BUNDLE_ENCODING_JSON)
	require.NoError(t, err)

	// Decode as a generic map so we catch regressions that would add a "locked"
	// field to the wire format — even if domain.BundleConfig doesn't model it.
	var raw map[string]any
	require.NoError(t, json.Unmarshal(payload, &raw))

	assert.NotContains(t, raw, "locked", "namespace bundle must not expose locked state")

	bundleConfigs, ok := raw["configs"].([]any)
	require.True(t, ok)
	require.Len(t, bundleConfigs, 2)

	for _, c := range bundleConfigs {
		entry, ok := c.(map[string]any)
		require.True(t, ok)
		assert.NotContains(t, entry, "locked", "bundle config must not carry per-config lock")
		assert.NotContains(t, entry, "namespaceLocked")
		assert.NotContains(t, entry, "namespace_locked")
	}
}
