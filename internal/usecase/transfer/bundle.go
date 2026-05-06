package transfer

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"github.com/sergeyslonimsky/elara/internal/domain"
	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
)

const (
	contentTypeJSON = "application/json"
	contentTypeYAML = "application/yaml"
	contentTypeZIP  = "application/zip"

	extJSON = ".json"
	extYAML = ".yaml"
	extZIP  = ".zip"
)

var errEmptyZip = errors.New("zip archive is empty")

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
	data, err := unzipIfNeeded(data)
	if err != nil {
		return nil, err
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
	data, err := unzipIfNeeded(data)
	if err != nil {
		return nil, err
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

// wrapInZip wraps payload bytes into a ZIP archive with a single entry.
func wrapInZip(filename string, data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	fw, err := zw.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("create zip entry: %w", err)
	}

	if _, err := fw.Write(data); err != nil {
		return nil, fmt.Errorf("write zip entry: %w", err)
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}

	return buf.Bytes(), nil
}

// unzipIfNeeded returns the first file's bytes if data is a ZIP archive, otherwise returns data unchanged.
func unzipIfNeeded(data []byte) ([]byte, error) {
	if !isZIP(data) {
		return data, nil
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open zip file %s: %w", f.Name, err)
		}

		content, readErr := io.ReadAll(rc)

		if closeErr := rc.Close(); closeErr != nil && readErr == nil {
			return nil, fmt.Errorf("close zip file %s: %w", f.Name, closeErr)
		}

		if readErr != nil {
			return nil, fmt.Errorf("read zip file %s: %w", f.Name, readErr)
		}

		return content, nil
	}

	return nil, errEmptyZip
}

func isZIP(data []byte) bool {
	return len(data) >= 4 &&
		data[0] == 0x50 && data[1] == 0x4B && data[2] == 0x03 && data[3] == 0x04
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
