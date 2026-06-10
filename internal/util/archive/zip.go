package archive

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
)

var ErrEmptyZip = errors.New("zip archive is empty")

// WrapInZip wraps payload bytes into a ZIP archive with a single entry.
func WrapInZip(filename string, data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)

	fw, err := writer.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("create zip entry: %w", err)
	}

	if _, err := fw.Write(data); err != nil {
		return nil, fmt.Errorf("write zip entry: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}

	return buf.Bytes(), nil
}

// UnzipIfNeeded returns the first file's bytes if data is a ZIP archive, otherwise returns data unchanged.
func UnzipIfNeeded(data []byte) ([]byte, error) {
	if !IsZIP(data) {
		return data, nil
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	for _, file := range zr.File {
		if file.FileInfo().IsDir() {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open zip file %s: %w", file.Name, err)
		}

		content, readErr := io.ReadAll(rc)

		if closeErr := rc.Close(); closeErr != nil && readErr == nil {
			return nil, fmt.Errorf("close zip file %s: %w", file.Name, closeErr)
		}

		if readErr != nil {
			return nil, fmt.Errorf("read zip file %s: %w", file.Name, readErr)
		}

		return content, nil
	}

	return nil, ErrEmptyZip
}

// IsZIP checks if data is a zip archive by its signature.
func IsZIP(data []byte) bool {
	return len(data) >= 4 &&
		data[0] == 0x50 && data[1] == 0x4B && data[2] == 0x03 && data[3] == 0x04
}
