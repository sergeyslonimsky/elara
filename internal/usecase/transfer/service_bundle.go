package transfer

import (
	"bytes"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/sergeyslonimsky/elara/internal/domain"
	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
	"github.com/sergeyslonimsky/elara/internal/util/archive"
)

const (
	contentTypeJSON = "application/json"
	contentTypeYAML = "application/yaml"
	contentTypeZIP  = "application/zip"

	extJSON = ".json"
	extYAML = ".yaml"
	extZIP  = ".zip"
)

func marshalBundle(v any, enc transferv1.BundleEncoding) ([]byte, string, error) {
	switch enc {
	case transferv1.BundleEncoding_BUNDLE_ENCODING_YAML:
		data, err := yaml.Marshal(v)
		if err != nil {
			return nil, "", fmt.Errorf("yaml marshal: %w", err)
		}

		return data, contentTypeYAML, nil
	default:
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return nil, "", fmt.Errorf("json marshal: %w", err)
		}

		return data, contentTypeJSON, nil
	}
}

// unmarshalAllBundle detects JSON/YAML/ZIP and returns an AllBundle.
func unmarshalAllBundle(data []byte) (*domain.AllBundle, error) {
	data, err := archive.UnzipIfNeeded(data)
	if err != nil {
		return nil, fmt.Errorf("unzip: %w", err)
	}

	var bundle domain.AllBundle

	if isYAML(data) {
		if err := yaml.Unmarshal(data, &bundle); err != nil {
			return nil, fmt.Errorf("yaml unmarshal bundle: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &bundle); err != nil {
			return nil, fmt.Errorf("json unmarshal bundle: %w", err)
		}
	}

	return &bundle, nil
}

// unmarshalNamespaceBundle detects JSON/YAML/ZIP and returns a NamespaceBundle.
func unmarshalNamespaceBundle(data []byte) (*domain.NamespaceBundle, error) {
	data, err := archive.UnzipIfNeeded(data)
	if err != nil {
		return nil, fmt.Errorf("unzip: %w", err)
	}

	var bundle domain.NamespaceBundle

	if isYAML(data) {
		if err := yaml.Unmarshal(data, &bundle); err != nil {
			return nil, fmt.Errorf("yaml unmarshal bundle: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &bundle); err != nil {
			return nil, fmt.Errorf("json unmarshal bundle: %w", err)
		}
	}

	return &bundle, nil
}

func isYAML(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return false
	}

	// JSON always starts with '{' or '['; anything else is treated as YAML.
	return trimmed[0] != '{' && trimmed[0] != '['
}

func bundleExtension(ct string, asZip bool) string {
	if asZip {
		return extZIP
	}

	switch ct {
	case contentTypeYAML:
		return extYAML
	default:
		return extJSON
	}
}
