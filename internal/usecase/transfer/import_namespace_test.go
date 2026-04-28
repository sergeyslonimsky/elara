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
	"go.uber.org/mock/gomock"
	"gopkg.in/yaml.v3"

	"github.com/sergeyslonimsky/elara/internal/domain"
	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
	"github.com/sergeyslonimsky/elara/internal/usecase/transfer"
	transfer_mock "github.com/sergeyslonimsky/elara/internal/usecase/transfer/mocks"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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

func newImportUC(
	getter *transfer_mock.MockimportConfigGetter,
	creator *transfer_mock.MockimportConfigCreator,
	updater *transfer_mock.MockimportConfigUpdater,
	nsGetter *transfer_mock.MockimportNSGetter,
	nsCreator *transfer_mock.MockimportNSCreator,
) *transfer.ImportNamespaceUseCase {
	return transfer.NewImportNamespaceUseCase(getter, creator, updater, nsGetter, nsCreator)
}

// ---------------------------------------------------------------------------
// Tests: targetNamespace="" auto-detect mode
// ---------------------------------------------------------------------------

func TestImportNamespaceUseCase_NamespaceBundleJSON_NewConfigs(t *testing.T) {
	t.Parallel()

	bundle := sampleBundle("my-ns")

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	nsGetter.EXPECT().Get(gomock.Any(), "my-ns").Return(nil, domain.ErrNotFound)
	nsCreator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	getter.EXPECT().Get(gomock.Any(), "/config.json", "my-ns").Return(nil, domain.ErrNotFound)
	creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		t.Context(),
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
}

func TestImportNamespaceUseCase_NamespaceBundleYAML_NewConfigs(t *testing.T) {
	t.Parallel()

	bundle := sampleBundle("my-ns")

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	nsGetter.EXPECT().Get(gomock.Any(), "my-ns").Return(nil, domain.ErrNotFound)
	nsCreator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	getter.EXPECT().Get(gomock.Any(), "/config.json", "my-ns").Return(nil, domain.ErrNotFound)
	creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		t.Context(),
		marshalNamespaceBundleYAML(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		false,
		"",
	)

	require.NoError(t, err)
	assert.Equal(t, 1, report.Created)
}

func TestImportNamespaceUseCase_NamespaceBundleZIP_NewConfigs(t *testing.T) {
	t.Parallel()

	bundle := sampleBundle("zip-ns")
	jsonData := marshalNamespaceBundle(t, bundle)
	zipped := wrapTestZip(t, "bundle.json", jsonData)

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	nsGetter.EXPECT().Get(gomock.Any(), "zip-ns").Return(nil, domain.ErrNotFound)
	nsCreator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	getter.EXPECT().Get(gomock.Any(), "/config.json", "zip-ns").Return(nil, domain.ErrNotFound)
	creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(t.Context(), zipped, transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP, false, "")

	require.NoError(t, err)
	assert.Equal(t, 1, report.Created)
}

func TestImportNamespaceUseCase_AllBundleJSON_MultipleNamespaces(t *testing.T) {
	t.Parallel()

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

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	nsGetter.EXPECT().Get(gomock.Any(), "ns1").Return(nil, domain.ErrNotFound)
	nsCreator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	nsGetter.EXPECT().Get(gomock.Any(), "ns2").Return(nil, domain.ErrNotFound)
	nsCreator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	getter.EXPECT().Get(gomock.Any(), "/a.json", "ns1").Return(nil, domain.ErrNotFound)
	getter.EXPECT().Get(gomock.Any(), "/b.json", "ns2").Return(nil, domain.ErrNotFound)
	getter.EXPECT().Get(gomock.Any(), "/c.yaml", "ns2").Return(nil, domain.ErrNotFound)
	creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).Times(3)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		t.Context(),
		marshalAllBundle(t, allBundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		false,
		"",
	)

	require.NoError(t, err)
	assert.Equal(t, 3, report.Created)
}

func TestImportNamespaceUseCase_AllBundleYAML(t *testing.T) {
	t.Parallel()

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

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	nsGetter.EXPECT().Get(gomock.Any(), "yaml-ns").Return(nil, domain.ErrNotFound)
	nsCreator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	getter.EXPECT().Get(gomock.Any(), "/cfg.json", "yaml-ns").Return(nil, domain.ErrNotFound)
	creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(t.Context(), yamlData, transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP, false, "")

	require.NoError(t, err)
	assert.Equal(t, 1, report.Created)
}

func TestImportNamespaceUseCase_ConflictResolutionSkip(t *testing.T) {
	t.Parallel()

	bundle := sampleBundle("my-ns")

	existingConfig := &domain.Config{
		Path:      "/config.json",
		Namespace: "my-ns",
		Content:   `{"old":"value"}`,
		Version:   1,
	}

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	nsGetter.EXPECT().Get(gomock.Any(), "my-ns").Return(&domain.Namespace{Name: "my-ns"}, nil)
	getter.EXPECT().Get(gomock.Any(), "/config.json", "my-ns").Return(existingConfig, nil)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		t.Context(),
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		false,
		"",
	)

	require.NoError(t, err)
	assert.Equal(t, 0, report.Created)
	assert.Equal(t, 1, report.Skipped)
}

func TestImportNamespaceUseCase_ConflictResolutionOverwrite(t *testing.T) {
	t.Parallel()

	bundle := sampleBundle("my-ns")

	existingConfig := &domain.Config{
		Path:      "/config.json",
		Namespace: "my-ns",
		Content:   `{"old":"value"}`,
		Version:   42,
	}

	var capturedConfig *domain.Config

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	nsGetter.EXPECT().Get(gomock.Any(), "my-ns").Return(&domain.Namespace{Name: "my-ns"}, nil)
	getter.EXPECT().Get(gomock.Any(), "/config.json", "my-ns").Return(existingConfig, nil)
	updater.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, cfg *domain.Config) error {
			capturedConfig = cfg

			return nil
		},
	)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		t.Context(),
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_OVERWRITE,
		false,
		"",
	)

	require.NoError(t, err)
	assert.Equal(t, 1, report.Updated)
	assert.Equal(t, 0, report.Skipped)

	// Verify version is preserved from existing config.
	require.NotNil(t, capturedConfig)
	assert.Equal(t, int64(42), capturedConfig.Version)
}

func TestImportNamespaceUseCase_ConflictResolutionFail(t *testing.T) {
	t.Parallel()

	bundle := sampleBundle("my-ns")

	existingConfig := &domain.Config{
		Path:      "/config.json",
		Namespace: "my-ns",
		Version:   1,
	}

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	nsGetter.EXPECT().Get(gomock.Any(), "my-ns").Return(&domain.Namespace{Name: "my-ns"}, nil)
	getter.EXPECT().Get(gomock.Any(), "/config.json", "my-ns").Return(existingConfig, nil)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		t.Context(),
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
}

func TestImportNamespaceUseCase_ConflictResolutionUnspecified_DefaultsToSkip(t *testing.T) {
	t.Parallel()

	bundle := sampleBundle("my-ns")

	existingConfig := &domain.Config{
		Path:      "/config.json",
		Namespace: "my-ns",
		Version:   1,
	}

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	nsGetter.EXPECT().Get(gomock.Any(), "my-ns").Return(&domain.Namespace{Name: "my-ns"}, nil)
	getter.EXPECT().Get(gomock.Any(), "/config.json", "my-ns").Return(existingConfig, nil)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		t.Context(),
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_UNSPECIFIED,
		false,
		"",
	)

	require.NoError(t, err)
	assert.Equal(t, 1, report.Skipped)
}

func TestImportNamespaceUseCase_DryRun(t *testing.T) {
	t.Parallel()

	bundle := sampleBundle("my-ns")

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	// Dry run: getter is called to check for conflicts, but no mutations happen.
	getter.EXPECT().Get(gomock.Any(), "/config.json", "my-ns").Return(nil, domain.ErrNotFound)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		t.Context(),
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		true,
		"",
	)

	require.NoError(t, err)
	assert.True(t, report.DryRun)
	assert.Equal(t, 1, report.Created)
}

func TestImportNamespaceUseCase_DryRun_OverwriteConflict(t *testing.T) {
	t.Parallel()

	bundle := sampleBundle("my-ns")

	existingConfig := &domain.Config{
		Path:      "/config.json",
		Namespace: "my-ns",
		Version:   1,
	}

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	// Dry run: getter called to check conflict, but updater must not be called.
	getter.EXPECT().Get(gomock.Any(), "/config.json", "my-ns").Return(existingConfig, nil)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		t.Context(),
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_OVERWRITE,
		true,
		"",
	)

	require.NoError(t, err)
	assert.True(t, report.DryRun)
	assert.Equal(t, 1, report.Updated)
}

func TestImportNamespaceUseCase_EmptyBundleNamespace_ValidationError(t *testing.T) {
	t.Parallel()

	// Bundle with no namespace field.
	bundle := domain.NamespaceBundle{
		Namespace:  "",
		ExportedAt: time.Now().UTC(),
		Configs:    []domain.BundleConfig{},
	}

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	_, err := uc.Execute(
		t.Context(),
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

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	_, err := uc.Execute(
		t.Context(),
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

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	// Empty data will unmarshal to an AllBundle with empty namespaces,
	// then fall back to NamespaceBundle with empty namespace field → validation error.
	_, err := uc.Execute(t.Context(), []byte(`{}`), transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP, false, "")

	require.Error(t, err)
	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
}

// ---------------------------------------------------------------------------
// Tests: targetNamespace specified (scoped mode)
// ---------------------------------------------------------------------------

func TestImportNamespaceUseCase_TargetNamespace_OverridesBundleNamespace(t *testing.T) {
	t.Parallel()

	// Bundle says "original-ns" but we want to import into "my-ns".
	bundle := sampleBundle("original-ns")

	var capturedNS *domain.Namespace
	var capturedConfig *domain.Config

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	nsGetter.EXPECT().Get(gomock.Any(), "my-ns").Return(nil, domain.ErrNotFound)
	nsCreator.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, ns *domain.Namespace) error {
			capturedNS = ns

			return nil
		},
	)
	getter.EXPECT().Get(gomock.Any(), "/config.json", "my-ns").Return(nil, domain.ErrNotFound)
	creator.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, cfg *domain.Config) error {
			capturedConfig = cfg

			return nil
		},
	)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	report, err := uc.Execute(
		t.Context(),
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		false,
		"my-ns",
	)

	require.NoError(t, err)
	assert.Equal(t, 1, report.Created)

	// Config should have been created under "my-ns", not "original-ns".
	require.NotNil(t, capturedConfig)
	assert.Equal(t, "my-ns", capturedConfig.Namespace)

	// Namespace ensured as "my-ns".
	require.NotNil(t, capturedNS)
	assert.Equal(t, "my-ns", capturedNS.Name)
}

func TestImportNamespaceUseCase_TargetNamespace_NamespaceAlreadyExists(t *testing.T) {
	t.Parallel()

	bundle := sampleBundle("original-ns")

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	nsGetter.EXPECT().Get(gomock.Any(), "target-ns").Return(&domain.Namespace{Name: "target-ns"}, nil)
	getter.EXPECT().Get(gomock.Any(), "/config.json", "target-ns").Return(nil, domain.ErrNotFound)
	creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	_, err := uc.Execute(
		t.Context(),
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		false,
		"target-ns",
	)

	require.NoError(t, err)
	// nsCreator.Create must not be called (verified by gomock controller — no EXPECT set)
}

func TestImportNamespaceUseCase_TargetNamespace_NamespaceNotFound_Creates(t *testing.T) {
	t.Parallel()

	bundle := domain.NamespaceBundle{
		Namespace:   "original-ns",
		Description: "the description from bundle",
		ExportedAt:  time.Now().UTC(),
		Configs:     []domain.BundleConfig{},
	}

	var capturedNS *domain.Namespace

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	nsGetter.EXPECT().Get(gomock.Any(), "new-ns").Return(nil, domain.ErrNotFound)
	nsCreator.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, ns *domain.Namespace) error {
			capturedNS = ns

			return nil
		},
	)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	_, err := uc.Execute(
		t.Context(),
		marshalNamespaceBundle(t, bundle),
		transferv1.ConflictResolution_CONFLICT_RESOLUTION_SKIP,
		false,
		"new-ns",
	)

	require.NoError(t, err)
	require.NotNil(t, capturedNS)
	assert.Equal(t, "new-ns", capturedNS.Name)
	// Description comes from the bundle.
	assert.Equal(t, "the description from bundle", capturedNS.Description)
}

func TestImportNamespaceUseCase_TargetNamespace_CorruptData_ValidationError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	_, err := uc.Execute(
		t.Context(),
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

	bundle := sampleBundle("my-ns")

	ctrl := gomock.NewController(t)
	getter := transfer_mock.NewMockimportConfigGetter(ctrl)
	creator := transfer_mock.NewMockimportConfigCreator(ctrl)
	updater := transfer_mock.NewMockimportConfigUpdater(ctrl)
	nsGetter := transfer_mock.NewMockimportNSGetter(ctrl)
	nsCreator := transfer_mock.NewMockimportNSCreator(ctrl)

	nsGetter.EXPECT().Get(gomock.Any(), "my-ns").Return(nil, errors.New("db error"))

	uc := newImportUC(getter, creator, updater, nsGetter, nsCreator)

	_, err := uc.Execute(
		t.Context(),
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
