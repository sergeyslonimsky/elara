package transfer_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/sergeyslonimsky/elara/internal/domain"
	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
	"github.com/sergeyslonimsky/elara/internal/usecase/transfer"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

type mockImportConfigGetter struct {
	configs map[string]*domain.Config // key: "namespace/path"
	err     error
}

func (m *mockImportConfigGetter) Get(_ context.Context, path, namespace string) (*domain.Config, error) {
	if m.err != nil {
		return nil, m.err
	}

	key := namespace + "/" + path
	if cfg, ok := m.configs[key]; ok {
		return cfg, nil
	}

	return nil, domain.ErrNotFound
}

type mockImportConfigCreator struct {
	created []*domain.Config
	err     error
	calls   int
}

func (m *mockImportConfigCreator) Create(_ context.Context, cfg *domain.Config) error {
	m.calls++
	if m.err != nil {
		return m.err
	}

	m.created = append(m.created, cfg)

	return nil
}

type mockImportConfigUpdater struct {
	updated []*domain.Config
	err     error
	calls   int
}

func (m *mockImportConfigUpdater) Update(_ context.Context, cfg *domain.Config) error {
	m.calls++
	if m.err != nil {
		return m.err
	}

	m.updated = append(m.updated, cfg)

	return nil
}

type mockImportNSGetter struct {
	namespaces map[string]*domain.Namespace
	err        error
	calls      int
}

func (m *mockImportNSGetter) Get(_ context.Context, name string) (*domain.Namespace, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}

	if ns, ok := m.namespaces[name]; ok {
		return ns, nil
	}

	return nil, domain.ErrNotFound
}

type mockImportNSCreator struct {
	created []*domain.Namespace
	err     error
	calls   int
}

func (m *mockImportNSCreator) Create(_ context.Context, ns *domain.Namespace) error {
	m.calls++
	if m.err != nil {
		return m.err
	}

	m.created = append(m.created, ns)

	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newImportUC(
	getter *mockImportConfigGetter,
	creator *mockImportConfigCreator,
	updater *mockImportConfigUpdater,
	nsGetter *mockImportNSGetter,
	nsCreator *mockImportNSCreator,
) *transfer.ImportNamespaceUseCase {
	return transfer.NewImportNamespaceUseCase(getter, creator, updater, nsGetter, nsCreator)
}

func marshalNamespaceBundle(t *testing.T, bundle domain.NamespaceBundle) []byte {
	t.Helper()

	data, err := json.Marshal(bundle)
	require.NoError(t, err)

	return data
}

func marshalNamespaceBundleYAML(t *testing.T, bundle domain.NamespaceBundle) []byte {
	t.Helper()

	data, err := yaml.Marshal(bundle)
	require.NoError(t, err)

	return data
}

func marshalAllBundle(t *testing.T, bundle domain.AllBundle) []byte {
	t.Helper()

	data, err := json.Marshal(bundle)
	require.NoError(t, err)

	return data
}

func sampleBundle(namespace string) domain.NamespaceBundle {
	return domain.NamespaceBundle{
		Namespace:   namespace,
		Description: "test namespace",
		ExportedAt:  time.Now().UTC(),
		Configs: []domain.BundleConfig{
			{Path: "/config.json", Content: `{"key":"value"}`, Format: domain.FormatJSON},
		},
	}
}

// ---------------------------------------------------------------------------
// Tests: targetNamespace="" auto-detect mode
// ---------------------------------------------------------------------------

func TestImportNamespaceUseCase_NamespaceBundleJSON_NewConfigs(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	bundle := sampleBundle("my-ns")

	getter := &mockImportConfigGetter{configs: map[string]*domain.Config{}}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{namespaces: map[string]*domain.Namespace{}}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		ctx,
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		false,
		"",
	)

	require.NoError(t, err)
	assert.Equal(t, 1, report.Created)
	assert.Equal(t, 0, report.Updated)
	assert.Equal(t, 0, report.Skipped)
	assert.Equal(t, 0, report.Failed)
	assert.Equal(t, 1, creator.calls)
}

func TestImportNamespaceUseCase_NamespaceBundleYAML_NewConfigs(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	bundle := sampleBundle("my-ns")

	getter := &mockImportConfigGetter{configs: map[string]*domain.Config{}}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{namespaces: map[string]*domain.Namespace{}}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		ctx,
		marshalNamespaceBundleYAML(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		false,
		"",
	)

	require.NoError(t, err)
	assert.Equal(t, 1, report.Created)
	assert.Equal(t, 1, creator.calls)
}

func TestImportNamespaceUseCase_NamespaceBundleZIP_NewConfigs(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	bundle := sampleBundle("zip-ns")

	jsonData := marshalNamespaceBundle(t, bundle)

	// Create a ZIP wrapping the JSON bundle.
	zipped := wrapTestZip(t, "bundle.json", jsonData)

	getter := &mockImportConfigGetter{configs: map[string]*domain.Config{}}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{namespaces: map[string]*domain.Namespace{}}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(ctx, zipped, transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP, false, "")

	require.NoError(t, err)
	assert.Equal(t, 1, report.Created)
	assert.Equal(t, 1, creator.calls)
}

func TestImportNamespaceUseCase_AllBundleJSON_MultipleNamespaces(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	allBundle := domain.AllBundle{
		ExportedAt: time.Now().UTC(),
		Namespaces: []domain.NamespaceBundle{
			{
				Namespace:   "ns1",
				Description: "first",
				ExportedAt:  time.Now().UTC(),
				Configs: []domain.BundleConfig{
					{Path: "/a.json", Content: `{}`, Format: domain.FormatJSON},
				},
			},
			{
				Namespace:   "ns2",
				Description: "second",
				ExportedAt:  time.Now().UTC(),
				Configs: []domain.BundleConfig{
					{Path: "/b.json", Content: `{}`, Format: domain.FormatJSON},
					{Path: "/c.yaml", Content: "key: val", Format: domain.FormatYAML},
				},
			},
		},
	}

	getter := &mockImportConfigGetter{configs: map[string]*domain.Config{}}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{namespaces: map[string]*domain.Namespace{}}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		ctx,
		marshalAllBundle(t, allBundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		false,
		"",
	)

	require.NoError(t, err)
	assert.Equal(t, 3, report.Created)
	assert.Equal(t, 3, creator.calls)
	// Both namespaces should have been ensured.
	assert.Equal(t, 2, nsGetter.calls)
	assert.Equal(t, 2, nsCreator.calls)
}

func TestImportNamespaceUseCase_AllBundleYAML(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	allBundle := domain.AllBundle{
		ExportedAt: time.Now().UTC(),
		Namespaces: []domain.NamespaceBundle{
			{
				Namespace:  "yaml-ns",
				ExportedAt: time.Now().UTC(),
				Configs: []domain.BundleConfig{
					{Path: "/cfg.json", Content: `{}`, Format: domain.FormatJSON},
				},
			},
		},
	}

	yamlData, err := yaml.Marshal(allBundle)
	require.NoError(t, err)

	getter := &mockImportConfigGetter{configs: map[string]*domain.Config{}}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{namespaces: map[string]*domain.Namespace{}}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(ctx, yamlData, transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP, false, "")

	require.NoError(t, err)
	assert.Equal(t, 1, report.Created)
}

func TestImportNamespaceUseCase_ConflictResolutionSkip(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	bundle := sampleBundle("my-ns")

	existingConfig := &domain.Config{
		Path:      "/config.json",
		Namespace: "my-ns",
		Content:   `{"old":"value"}`,
		Version:   1,
	}
	getter := &mockImportConfigGetter{
		configs: map[string]*domain.Config{
			"my-ns//config.json": existingConfig,
		},
	}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{namespaces: map[string]*domain.Namespace{"my-ns": {Name: "my-ns"}}}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		ctx,
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		false,
		"",
	)

	require.NoError(t, err)
	assert.Equal(t, 0, report.Created)
	assert.Equal(t, 1, report.Skipped)
	assert.Equal(t, 0, creator.calls)
	assert.Equal(t, 0, updater.calls)
}

func TestImportNamespaceUseCase_ConflictResolutionOverwrite(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	bundle := sampleBundle("my-ns")

	existingConfig := &domain.Config{
		Path:      "/config.json",
		Namespace: "my-ns",
		Content:   `{"old":"value"}`,
		Version:   42,
	}
	getter := &mockImportConfigGetter{
		configs: map[string]*domain.Config{
			"my-ns//config.json": existingConfig,
		},
	}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{namespaces: map[string]*domain.Namespace{"my-ns": {Name: "my-ns"}}}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		ctx,
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_OVERWRITE,
		false,
		"",
	)

	require.NoError(t, err)
	assert.Equal(t, 1, report.Updated)
	assert.Equal(t, 0, report.Skipped)
	assert.Equal(t, 1, updater.calls)
	assert.Equal(t, 0, creator.calls)

	// Verify version is preserved from existing config.
	require.Len(t, updater.updated, 1)
	assert.Equal(t, int64(42), updater.updated[0].Version)
}

func TestImportNamespaceUseCase_ConflictResolutionFail(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	bundle := sampleBundle("my-ns")

	existingConfig := &domain.Config{
		Path:      "/config.json",
		Namespace: "my-ns",
		Version:   1,
	}
	getter := &mockImportConfigGetter{
		configs: map[string]*domain.Config{
			"my-ns//config.json": existingConfig,
		},
	}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{namespaces: map[string]*domain.Namespace{"my-ns": {Name: "my-ns"}}}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		ctx,
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_FAIL,
		false,
		"",
	)

	require.NoError(t, err)
	assert.Equal(t, 1, report.Failed)
	require.Len(t, report.Errors, 1)
	assert.Equal(t, "/config.json", report.Errors[0].Path)
	assert.Equal(t, "my-ns", report.Errors[0].Namespace)
	assert.Equal(t, 0, creator.calls)
	assert.Equal(t, 0, updater.calls)
}

func TestImportNamespaceUseCase_ConflictResolutionUnspecified_DefaultsToSkip(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	bundle := sampleBundle("my-ns")

	existingConfig := &domain.Config{
		Path:      "/config.json",
		Namespace: "my-ns",
		Version:   1,
	}
	getter := &mockImportConfigGetter{
		configs: map[string]*domain.Config{
			"my-ns//config.json": existingConfig,
		},
	}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{namespaces: map[string]*domain.Namespace{"my-ns": {Name: "my-ns"}}}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		ctx,
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_UNSPECIFIED,
		false,
		"",
	)

	require.NoError(t, err)
	assert.Equal(t, 1, report.Skipped)
	assert.Equal(t, 0, creator.calls)
}

func TestImportNamespaceUseCase_DryRun(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	bundle := sampleBundle("my-ns")

	getter := &mockImportConfigGetter{configs: map[string]*domain.Config{}}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{namespaces: map[string]*domain.Namespace{}}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		ctx,
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		true,
		"",
	)

	require.NoError(t, err)
	assert.True(t, report.DryRun)
	assert.Equal(t, 1, report.Created)
	// Neither creator, updater, nor nsCreator should be called during dry run.
	assert.Equal(t, 0, creator.calls)
	assert.Equal(t, 0, updater.calls)
	assert.Equal(t, 0, nsGetter.calls)
	assert.Equal(t, 0, nsCreator.calls)
}

func TestImportNamespaceUseCase_DryRun_OverwriteConflict(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	bundle := sampleBundle("my-ns")

	existingConfig := &domain.Config{
		Path:      "/config.json",
		Namespace: "my-ns",
		Version:   1,
	}
	getter := &mockImportConfigGetter{
		configs: map[string]*domain.Config{
			"my-ns//config.json": existingConfig,
		},
	}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{namespaces: map[string]*domain.Namespace{}}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		ctx,
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_OVERWRITE,
		true,
		"",
	)

	require.NoError(t, err)
	assert.True(t, report.DryRun)
	assert.Equal(t, 1, report.Updated)
	assert.Equal(t, 0, updater.calls)
	assert.Equal(t, 0, creator.calls)
}

func TestImportNamespaceUseCase_EmptyBundleNamespace_ValidationError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Bundle with no namespace field.
	bundle := domain.NamespaceBundle{
		Namespace:  "",
		ExportedAt: time.Now().UTC(),
		Configs:    []domain.BundleConfig{},
	}

	getter := &mockImportConfigGetter{configs: map[string]*domain.Config{}}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{namespaces: map[string]*domain.Namespace{}}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	_, err := uc.Execute(
		ctx,
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		false,
		"",
	)

	require.Error(t, err)
	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "namespace", ve.Field)
}

func TestImportNamespaceUseCase_CorruptJSON_ValidationError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	getter := &mockImportConfigGetter{configs: map[string]*domain.Config{}}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{namespaces: map[string]*domain.Namespace{}}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	_, err := uc.Execute(
		ctx,
		[]byte(`{corrupt json`),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		false,
		"",
	)

	require.Error(t, err)
	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "data", ve.Field)
}

func TestImportNamespaceUseCase_EmptyData_ValidationError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	getter := &mockImportConfigGetter{configs: map[string]*domain.Config{}}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{namespaces: map[string]*domain.Namespace{}}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	// Empty data will unmarshal to an AllBundle with empty namespaces,
	// then fall back to NamespaceBundle with empty namespace field → validation error.
	_, err := uc.Execute(ctx, []byte(`{}`), transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP, false, "")

	require.Error(t, err)
	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
}

// ---------------------------------------------------------------------------
// Tests: targetNamespace specified (scoped mode)
// ---------------------------------------------------------------------------

func TestImportNamespaceUseCase_TargetNamespace_OverridesBundleNamespace(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Bundle says "original-ns" but we want to import into "my-ns".
	bundle := sampleBundle("original-ns")

	getter := &mockImportConfigGetter{configs: map[string]*domain.Config{}}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{namespaces: map[string]*domain.Namespace{}}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		ctx,
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		false,
		"my-ns",
	)

	require.NoError(t, err)
	assert.Equal(t, 1, report.Created)

	// Config should have been created under "my-ns", not "original-ns".
	require.Len(t, creator.created, 1)
	assert.Equal(t, "my-ns", creator.created[0].Namespace)

	// Namespace ensured as "my-ns".
	assert.Equal(t, 1, nsGetter.calls)
	assert.Equal(t, 1, nsCreator.calls)
	require.Len(t, nsCreator.created, 1)
	assert.Equal(t, "my-ns", nsCreator.created[0].Name)
}

func TestImportNamespaceUseCase_TargetNamespace_NamespaceAlreadyExists(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	bundle := sampleBundle("original-ns")

	getter := &mockImportConfigGetter{configs: map[string]*domain.Config{}}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{
		namespaces: map[string]*domain.Namespace{
			"target-ns": {Name: "target-ns"},
		},
	}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	_, err := uc.Execute(
		ctx,
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		false,
		"target-ns",
	)

	require.NoError(t, err)
	assert.Equal(t, 1, nsGetter.calls)
	// Namespace already exists, so no creation.
	assert.Equal(t, 0, nsCreator.calls)
}

func TestImportNamespaceUseCase_TargetNamespace_NamespaceNotFound_Creates(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	bundle := domain.NamespaceBundle{
		Namespace:   "original-ns",
		Description: "the description from bundle",
		ExportedAt:  time.Now().UTC(),
		Configs:     []domain.BundleConfig{},
	}

	getter := &mockImportConfigGetter{configs: map[string]*domain.Config{}}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{namespaces: map[string]*domain.Namespace{}}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	_, err := uc.Execute(
		ctx,
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		false,
		"new-ns",
	)

	require.NoError(t, err)
	assert.Equal(t, 1, nsCreator.calls)
	require.Len(t, nsCreator.created, 1)
	assert.Equal(t, "new-ns", nsCreator.created[0].Name)
	// Description comes from the bundle.
	assert.Equal(t, "the description from bundle", nsCreator.created[0].Description)
}

func TestImportNamespaceUseCase_TargetNamespace_CorruptData_ValidationError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	getter := &mockImportConfigGetter{configs: map[string]*domain.Config{}}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{namespaces: map[string]*domain.Namespace{}}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	_, err := uc.Execute(
		ctx,
		[]byte(`{corrupt`),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		false,
		"my-ns",
	)

	require.Error(t, err)
	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "data", ve.Field)
}

// ---------------------------------------------------------------------------
// Tests: ensureNamespace error propagation
// ---------------------------------------------------------------------------

func TestImportNamespaceUseCase_NSGetterError_Propagated(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	bundle := sampleBundle("my-ns")

	getter := &mockImportConfigGetter{configs: map[string]*domain.Config{}}
	creator := &mockImportConfigCreator{}
	updater := &mockImportConfigUpdater{}
	nsGetter := &mockImportNSGetter{err: errors.New("db error")}
	nsCreator := &mockImportNSCreator{}

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	_, err := uc.Execute(
		ctx,
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		false,
		"",
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ensure namespace")
}

// ---------------------------------------------------------------------------
// Test helper: wrapTestZip creates a single-file ZIP in memory.
// ---------------------------------------------------------------------------

func wrapTestZip(t *testing.T, filename string, data []byte) []byte {
	t.Helper()

	zipBytes, err := wrapTestZipBytes(filename, data)
	require.NoError(t, err)

	return zipBytes
}

func wrapTestZipBytes(filename string, data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	fw, err := zw.Create(filename)
	if err != nil {
		return nil, err
	}

	if _, err := fw.Write(data); err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
