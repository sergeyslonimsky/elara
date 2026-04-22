package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

type Format string

const (
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
	FormatOther Format = "other"
)

func ParseFormat(s string) (Format, error) {
	switch s {
	case "json":
		return FormatJSON, nil
	case "yaml", "yml":
		return FormatYAML, nil
	case "other":
		return FormatOther, nil
	default:
		return "", NewInvalidFormatError(s)
	}
}

func DetectFormatFromPath(path string) Format {
	switch {
	case strings.HasSuffix(path, ".json"):
		return FormatJSON
	case strings.HasSuffix(path, ".yaml"), strings.HasSuffix(path, ".yml"):
		return FormatYAML
	default:
		return FormatOther
	}
}

func ValidatePath(path string) error {
	if path == "" {
		return NewValidationError("path", "path is required")
	}

	if !strings.HasPrefix(path, "/") {
		return NewValidationError("path", "path must start with /")
	}

	if strings.Contains(path, "//") {
		return NewValidationError("path", "path must not contain //")
	}

	if strings.HasSuffix(path, "/") {
		return NewValidationError("path", "path must not end with /")
	}

	return nil
}

func (f Format) String() string {
	return string(f)
}

type Config struct {
	Path            string
	Content         string
	ContentHash     string
	Format          Format
	Version         int64
	Revision        int64 // mod_revision: global revision when last modified
	CreateRevision  int64 // global revision when first created (etcd compat)
	Namespace       string
	Metadata        map[string]string
	Locked          bool
	NamespaceLocked bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ConfigSummary struct {
	Path            string
	ContentHash     string
	Format          Format
	Version         int64
	Revision        int64
	Namespace       string
	Metadata        map[string]string
	Locked          bool
	NamespaceLocked bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (c *Config) ToSummary() *ConfigSummary {
	return &ConfigSummary{
		Path:            c.Path,
		ContentHash:     c.ContentHash,
		Format:          c.Format,
		Version:         c.Version,
		Revision:        c.Revision,
		Namespace:       c.Namespace,
		Metadata:        c.Metadata,
		Locked:          c.Locked,
		NamespaceLocked: c.NamespaceLocked,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
	}
}

func (c *Config) GenerateHash() {
	hash := sha256.Sum256([]byte(c.Content))
	c.ContentHash = hex.EncodeToString(hash[:])
}

func (c *Config) SetDefaults() {
	if c.Metadata == nil {
		c.Metadata = make(map[string]string)
	}
}

func (c *Config) HasContentChanged(newContent string) bool {
	hash := sha256.Sum256([]byte(newContent))

	return c.ContentHash != hex.EncodeToString(hash[:])
}

type ConfigKey struct {
	Path      string
	Namespace string
}

func (c *Config) Key() ConfigKey {
	return ConfigKey{
		Path:      c.Path,
		Namespace: c.Namespace,
	}
}
