package transfer

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/util/archive"
)

const exportFileName = "elara-export-all"

func (s *Service) ExportAll(
	ctx context.Context,
	asZip bool,
	enc transferv1.BundleEncoding,
	layout transferv1.ZipLayout,
) ([]byte, string, string, error) {
	allBundle, err := s.buildAllBundle(ctx)
	if err != nil {
		return nil, "", "", err
	}

	if asZip && layout == transferv1.ZipLayout_ZIP_LAYOUT_PER_NAMESPACE {
		payload, err := marshalPerNamespaceZip(allBundle, enc)
		if err != nil {
			return nil, "", "", err
		}

		return payload, contentTypeZIP, exportFileName + "." + extZIP, nil
	}

	payload, ct, err := marshalBundle(allBundle, enc)
	if err != nil {
		return nil, "", "", err
	}

	ext := bundleExtension(ct, asZip)
	fname := exportFileName + ext

	if asZip {
		innerName := exportFileName + bundleExtension(ct, false)

		payload, err = archive.WrapInZip(innerName, payload)
		if err != nil {
			return nil, "", "", fmt.Errorf("wrap in zip: %w", err)
		}

		ct = contentTypeZIP
	}

	return payload, ct, fname, nil
}

func (s *Service) ExportNamespace(
	ctx context.Context,
	namespace string,
	asZip bool,
	enc transferv1.BundleEncoding,
) ([]byte, string, string, error) {
	ns, err := s.namespaces.Get(ctx, namespace)
	if err != nil {
		return nil, "", "", fmt.Errorf("get namespace: %w", err)
	}

	configsByNS, err := s.configs.ListAllByNamespace(ctx, namespace)
	if err != nil {
		return nil, "", "", fmt.Errorf("list configs: %w", err)
	}

	bundle := domain.NamespaceBundle{
		Namespace:   namespace,
		Description: ns.Description,
		ExportedAt:  time.Now().UTC(),
		Configs:     make([]domain.BundleConfig, 0, len(configsByNS)),
	}

	for _, cfg := range configsByNS {
		bundle.Configs = append(bundle.Configs, domain.BundleConfig{
			Path:     cfg.Path,
			Content:  cfg.Content,
			Format:   cfg.Format,
			Metadata: cfg.Metadata,
		})
	}

	sort.Slice(bundle.Configs, func(i, j int) bool {
		return bundle.Configs[i].Path < bundle.Configs[j].Path
	})

	payload, ct, err := marshalBundle(bundle, enc)
	if err != nil {
		return nil, "", "", err
	}

	ext := bundleExtension(ct, asZip)
	fname := namespace + "-export" + ext

	if asZip {
		innerName := namespace + "-export" + bundleExtension(ct, false)

		payload, err = archive.WrapInZip(innerName, payload)
		if err != nil {
			return nil, "", "", fmt.Errorf("wrap in zip: %w", err)
		}

		ct = contentTypeZIP
	}

	return payload, ct, fname, nil
}

func (s *Service) buildAllBundle(ctx context.Context) (domain.AllBundle, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return domain.AllBundle{}, domain.ErrUnauthorized
	}

	ns, err := s.namespaces.ListAll(ctx)
	if err != nil {
		return domain.AllBundle{}, fmt.Errorf("list namespaces: %w", err)
	}

	exportedAt := time.Now().UTC()
	allBundle := domain.AllBundle{
		ExportedAt: exportedAt,
		Namespaces: make([]domain.NamespaceBundle, 0, len(ns)),
	}

	for _, ns := range ns {
		if !s.pdp.Has(claims.Email, domain.Permission{
			Object: domain.ObjectNamespace,
			Action: domain.ActionRead,
			Domain: ns.Name,
		}) {
			continue
		}

		nsBundle, err := s.buildNamespaceBundle(ctx, ns, exportedAt)
		if err != nil {
			return domain.AllBundle{}, err
		}

		allBundle.Namespaces = append(allBundle.Namespaces, nsBundle)
	}

	// Deterministic order: API contract should not depend on storage iteration order.
	sort.Slice(allBundle.Namespaces, func(i, j int) bool {
		return allBundle.Namespaces[i].Namespace < allBundle.Namespaces[j].Namespace
	})

	return allBundle, nil
}

func (s *Service) buildNamespaceBundle(
	ctx context.Context,
	ns *domain.Namespace,
	exportedAt time.Time,
) (domain.NamespaceBundle, error) {
	configs, err := s.configs.ListAllByNamespace(ctx, ns.Name)
	if err != nil {
		return domain.NamespaceBundle{}, fmt.Errorf(
			"list configs for namespace %s: %w",
			ns.Name,
			err,
		)
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

	sort.Slice(nsBundle.Configs, func(i, j int) bool {
		return nsBundle.Configs[i].Path < nsBundle.Configs[j].Path
	})

	return nsBundle, nil
}

// marshalPerNamespaceZip creates a ZIP with index.json/yaml plus one file per namespace.
func marshalPerNamespaceZip(
	bundle domain.AllBundle,
	enc transferv1.BundleEncoding,
) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for i := range bundle.Namespaces {
		if err := writeZipNamespace(zw, &bundle.Namespaces[i], enc); err != nil {
			if cErr := zw.Close(); cErr != nil {
				slog.Error(
					"close namespace zip writer",
					slog.String("namespace", bundle.Namespaces[i].Namespace),
					slog.Any("error", cErr),
				)
			}

			return nil, err
		}
	}

	if err := writeZipIndex(zw, bundle, enc); err != nil {
		if cErr := zw.Close(); cErr != nil {
			slog.Error("close index zip writer", slog.Any("error", cErr))
		}

		return nil, err
	}

	if err := zw.Close(); err != nil {
		slog.Error("close zip body", slog.Any("error", err))
	}

	return buf.Bytes(), nil
}

func writeZipNamespace(
	zw *zip.Writer,
	ns *domain.NamespaceBundle,
	enc transferv1.BundleEncoding,
) error {
	payload, ct, err := marshalBundle(ns, enc)
	if err != nil {
		return fmt.Errorf("marshal namespace %s: %w", ns.Namespace, err)
	}

	ext := extJSON
	if ct == contentTypeYAML {
		ext = extYAML
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
