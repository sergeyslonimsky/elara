package pathutil

import "strings"

// Normalize normalizes a path by ensuring it starts with a slash and does not end with one.
func Normalize(path string) string {
	if path == "" {
		return "/"
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return strings.TrimSuffix(path, "/")
}
