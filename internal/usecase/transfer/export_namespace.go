package transfer

import (
	"context"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
)

//go:generate mockgen -destination=mocks/mock_export_namespace.go -package=transfer_mock . exportNSConfigLister,exportNSChecker

type exportNSConfigLister interface {
	ListAllByNamespace(ctx context.Context, namespace string) ([]*domain.Config, error)
}

type exportNSChecker interface {
	Get(ctx context.Context, name string) (*domain.Namespace, error)
}

type ExportNamespaceUseCase struct {
	configs    exportNSConfigLister
	namespaces exportNSChecker
}

func NewExportNamespaceUseCase(configs exportNSConfigLister, namespaces exportNSChecker) *ExportNamespaceUseCase {
	return &ExportNamespaceUseCase{configs: configs, namespaces: namespaces}
}

func (uc *ExportNamespaceUseCase) Execute(
	ctx context.Context,
	namespace string,
	asZip bool,
	enc transferv1.BundleEncoding,
) ([]byte, string, string, error) {
	ns, err := uc.namespaces.Get(ctx, namespace)
	if err != nil {
		return nil, "", "", fmt.Errorf("get namespace: %w", err)
	}

	configs, err := uc.configs.ListAllByNamespace(ctx, namespace)
	if err != nil {
		return nil, "", "", fmt.Errorf("list configs: %w", err)
	}

	bundle := domain.NamespaceBundle{
		Namespace:   namespace,
		Description: ns.Description,
		ExportedAt:  time.Now().UTC(),
		Configs:     make([]domain.BundleConfig, 0, len(configs)),
	}

	for _, cfg := range configs {
		bundle.Configs = append(bundle.Configs, domain.BundleConfig{
			Path:     cfg.Path,
			Content:  cfg.Content,
			Format:   cfg.Format,
			Metadata: cfg.Metadata,
		})
	}

	payload, ct, err := marshalBundle(bundle, enc)
	if err != nil {
		return nil, "", "", err
	}

	ext := bundleExtension(ct, asZip)
	fname := namespace + "-export" + ext

	if asZip {
		innerName := namespace + "-export" + bundleExtension(ct, false)

		payload, err = wrapInZip(innerName, payload)
		if err != nil {
			return nil, "", "", err
		}

		ct = contentTypeZIP
	}

	return payload, ct, fname, nil
}
