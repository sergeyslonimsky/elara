package transfer

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
)

type exportAllConfigLister interface {
	ListAllByNamespace(ctx context.Context, namespace string) ([]*domain.Config, error)
}

type exportAllNSLister interface {
	List(ctx context.Context) ([]*domain.Namespace, error)
}

type ExportAllUseCase struct {
	configs    exportAllConfigLister
	namespaces exportAllNSLister
}

func NewExportAllUseCase(configs exportAllConfigLister, namespaces exportAllNSLister) *ExportAllUseCase {
	return &ExportAllUseCase{configs: configs, namespaces: namespaces}
}

func (uc *ExportAllUseCase) Execute(
	ctx context.Context,
	asZip bool,
	enc transferv1.BundleEncoding,
	layout transferv1.ZipLayout,
) ([]byte, string, string, error) {
	allBundle, err := uc.buildAllBundle(ctx)
	if err != nil {
		return nil, "", "", err
	}

	if asZip && layout == transferv1.ZipLayout_ZIP_LAYOUT_PER_NAMESPACE {
		payload, err := marshalPerNamespaceZip(allBundle, enc)
		if err != nil {
			return nil, "", "", err
		}

		return payload, contentTypeZIP, "elara-export-all.zip", nil
	}

	payload, ct, err := marshalBundle(allBundle, enc)
	if err != nil {
		return nil, "", "", err
	}

	ext := bundleExtension(ct, asZip)
	fname := "elara-export-all" + ext

	if asZip {
		innerName := "elara-export-all" + bundleExtension(ct, false)

		payload, err = wrapInZip(innerName, payload)
		if err != nil {
			return nil, "", "", err
		}

		ct = contentTypeZIP
	}

	return payload, ct, fname, nil
}

func (uc *ExportAllUseCase) buildAllBundle(ctx context.Context) (domain.AllBundle, error) {
	namespaces, err := uc.namespaces.List(ctx)
	if err != nil {
		return domain.AllBundle{}, fmt.Errorf("list namespaces: %w", err)
	}

	exportedAt := time.Now().UTC()
	allBundle := domain.AllBundle{
		ExportedAt: exportedAt,
		Namespaces: make([]domain.NamespaceBundle, 0, len(namespaces)),
	}

	for _, ns := range namespaces {
		nsBundle, err := uc.buildNamespaceBundle(ctx, ns, exportedAt)
		if err != nil {
			return domain.AllBundle{}, err
		}

		allBundle.Namespaces = append(allBundle.Namespaces, nsBundle)
	}

	return allBundle, nil
}

func (uc *ExportAllUseCase) buildNamespaceBundle(
	ctx context.Context,
	ns *domain.Namespace,
	exportedAt time.Time,
) (domain.NamespaceBundle, error) {
	configs, err := uc.configs.ListAllByNamespace(ctx, ns.Name)
	if err != nil {
		return domain.NamespaceBundle{}, fmt.Errorf("list configs for namespace %s: %w", ns.Name, err)
	}

	nsBundle := domain.NamespaceBundle{
		Namespace:   ns.Name,
		Description: ns.Description,
		ExportedAt:  exportedAt,
		Configs:     make([]domain.BundleConfig, 0, len(configs)),
	}

	for _, cfg := range configs {
		nsBundle.Configs = append(nsBundle.Configs, domain.BundleConfig{
			Path:     cfg.Path,
			Content:  cfg.Content,
			Format:   cfg.Format,
			Metadata: cfg.Metadata,
		})
	}

	return nsBundle, nil
}

// marshalPerNamespaceZip creates a ZIP with index.json/yaml plus one file per namespace.
func marshalPerNamespaceZip(bundle domain.AllBundle, enc transferv1.BundleEncoding) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for i := range bundle.Namespaces {
		if err := writeZipNamespace(zw, &bundle.Namespaces[i], enc); err != nil {
			return nil, err
		}
	}

	if err := writeZipIndex(zw, bundle, enc); err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}

	return buf.Bytes(), nil
}

func writeZipNamespace(zw *zip.Writer, ns *domain.NamespaceBundle, enc transferv1.BundleEncoding) error {
	payload, ct, err := marshalBundle(ns, enc)
	if err != nil {
		return fmt.Errorf("marshal namespace %s: %w", ns.Namespace, err)
	}

	ext := ".json"
	if ct == contentTypeYAML {
		ext = ".yaml"
	}

	fname := "namespaces/" + ns.Namespace + ext

	fw, err := zw.Create(fname)
	if err != nil {
		return fmt.Errorf("create zip entry %s: %w", fname, err)
	}

	if _, err := fw.Write(payload); err != nil {
		return fmt.Errorf("write zip entry %s: %w", fname, err)
	}

	return nil
}

func writeZipIndex(zw *zip.Writer, bundle domain.AllBundle, enc transferv1.BundleEncoding) error {
	type index struct {
		ExportedAt time.Time `json:"exportedAt" yaml:"exportedAt"`
		Namespaces []string  `json:"namespaces" yaml:"namespaces"`
	}

	names := make([]string, 0, len(bundle.Namespaces))
	for _, ns := range bundle.Namespaces {
		names = append(names, ns.Namespace)
	}

	idx := index{ExportedAt: bundle.ExportedAt, Namespaces: names}

	idxPayload, ct, err := marshalBundle(idx, enc)
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}

	idxName := "index.json"
	if ct == contentTypeYAML {
		idxName = "index.yaml"
	}

	fw, err := zw.Create(idxName)
	if err != nil {
		return fmt.Errorf("create index zip entry: %w", err)
	}

	if _, err := fw.Write(idxPayload); err != nil {
		return fmt.Errorf("write index: %w", err)
	}

	return nil
}
