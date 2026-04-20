package domain

import "time"

type BundleConfig struct {
	Path     string            `json:"path"               yaml:"path"`
	Content  string            `json:"content"            yaml:"content"`
	Format   Format            `json:"format"             yaml:"format"`
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type NamespaceBundle struct {
	Namespace  string         `json:"namespace"  yaml:"namespace"`
	ExportedAt time.Time      `json:"exportedAt" yaml:"exportedAt"`
	Configs    []BundleConfig `json:"configs"    yaml:"configs"`
}

type AllBundle struct {
	ExportedAt time.Time         `json:"exportedAt" yaml:"exportedAt"`
	Namespaces []NamespaceBundle `json:"namespaces" yaml:"namespaces"`
}

type ImportReport struct {
	Created int
	Skipped int
	Failed  int
	Errors  []BundleImportError
	DryRun  bool
}

type BundleImportError struct {
	Path      string
	Namespace string
	Message   string
}

func (r *ImportReport) AddError(path, namespace, message string) {
	r.Failed++
	r.Errors = append(r.Errors, BundleImportError{Path: path, Namespace: namespace, Message: message})
}
