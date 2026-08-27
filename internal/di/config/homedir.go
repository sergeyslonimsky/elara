package config

import (
	"os"
	"path/filepath"
)

// ElaraHomeDir returns the base directory Elara uses for local,
// non-containerized installs (a `go install`'d or downloaded binary run
// straight from a shell) — analogous to `~/.docker` or `~/.kube` for other
// CLI-shaped tools.
//
// It is deliberately NOT consulted inside a container: the Docker image
// always sets CONFIG_DATA_PATH explicitly (to /var/lib/elara), and callers
// here only use this as a fallback default when no path was configured.
//
// Returns "" when no suitable directory can be determined, so callers must
// fall back to a relative default (e.g. "./data") rather than panicking.
func ElaraHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".elara")
}
