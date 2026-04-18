package ui

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// NewHandler creates an HTTP handler that serves static files from the given filesystem
// with SPA (Single Page Application) fallback: non-file requests serve index.html.
func NewHandler(fileSystem fs.FS) http.Handler {
	return &handler{fs: fileSystem}
}

type handler struct {
	fs fs.FS
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only handle GET/HEAD.
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)

		return
	}

	reqPath := strings.TrimPrefix(r.URL.Path, "/")
	if reqPath == "" {
		reqPath = "index.html"
	}

	// Try to serve the exact file.
	if h.serveFile(w, r, reqPath) {
		return
	}

	// SPA fallback: serve index.html for any path that doesn't match a static file.
	h.serveFile(w, r, "index.html")
}

func (h *handler) serveFile(w http.ResponseWriter, r *http.Request, filePath string) bool {
	file, err := h.fs.Open(filePath)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		return false
	}

	setContentType(w, filePath)

	if seeker, ok := file.(io.ReadSeeker); ok {
		http.ServeContent(w, r, filePath, info.ModTime(), seeker)
	} else {
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, file)
	}

	return true
}

func setContentType(w http.ResponseWriter, filename string) {
	ext := strings.ToLower(path.Ext(filename))

	contentTypes := map[string]string{
		".html":  "text/html; charset=utf-8",
		".css":   "text/css; charset=utf-8",
		".js":    "application/javascript; charset=utf-8",
		".mjs":   "application/javascript; charset=utf-8",
		".json":  "application/json; charset=utf-8",
		".png":   "image/png",
		".jpg":   "image/jpeg",
		".jpeg":  "image/jpeg",
		".gif":   "image/gif",
		".svg":   "image/svg+xml; charset=utf-8",
		".ico":   "image/x-icon",
		".woff":  "font/woff",
		".woff2": "font/woff2",
		".ttf":   "font/ttf",
	}

	if ct, ok := contentTypes[ext]; ok {
		w.Header().Set("Content-Type", ct)
	}
}
