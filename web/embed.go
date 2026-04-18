package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFiles embed.FS

// DistFS returns the frontend static files filesystem.
// Returns nil if dist directory is empty (dev mode).
func DistFS() fs.FS {
	sub, err := fs.Sub(distFiles, "dist")
	if err != nil {
		return nil
	}

	return sub
}
