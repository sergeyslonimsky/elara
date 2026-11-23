package domain

import (
	"regexp"
	"strings"
)

const (
	// TokenUsernameSuffixLen is the length of the random part the server
	// appends to a token's name prefix to form its Username.
	TokenUsernameSuffixLen = 6

	// MaxTokenNamePrefixLen caps the user-chosen part of a Username so the
	// whole login (prefix + "-" + suffix) stays short enough to type.
	MaxTokenNamePrefixLen = 48

	// TokenUsernameFallbackPrefix is used when a legacy free-form name
	// sanitizes to nothing (e.g. it was written entirely in Cyrillic).
	TokenUsernameFallbackPrefix = "token"

	tokenUsernameSeparator = "-"
)

// tokenNamePrefixPattern: lowercase ASCII letters, digits, '.', '_' and '-',
// starting and ending with a letter or digit. ':' is excluded on purpose —
// `etcdctl --user name:password` splits on the first colon.
var tokenNamePrefixPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9._-]*[a-z0-9])?$`)

// ValidateTokenNamePrefix checks the user-chosen prefix of a token Username.
func ValidateTokenNamePrefix(prefix string) error {
	if prefix == "" {
		return NewValidationError("name", "name prefix is required")
	}

	if len(prefix) > MaxTokenNamePrefixLen {
		return NewValidationError("name", "name prefix must be at most 48 characters")
	}

	if !tokenNamePrefixPattern.MatchString(prefix) {
		return NewValidationError(
			"name",
			"name prefix may contain only lowercase letters, digits, '.', '_' and '-', "+
				"and must start and end with a letter or digit",
		)
	}

	return nil
}

// NewTokenUsername joins a validated prefix and a server-generated suffix
// into the login an etcd client presents as its user name.
func NewTokenUsername(prefix, suffix string) string {
	return prefix + tokenUsernameSeparator + suffix
}

// SanitizeTokenNamePrefix turns a legacy free-form token name into a string
// that passes ValidateTokenNamePrefix. It is used once, to give tokens created
// before Username existed a login derived from their old name.
func SanitizeTokenNamePrefix(name string) string {
	// Lowercase, keep [a-z0-9._], turn every other rune (spaces, ':', '/',
	// non-ASCII letters, …) into a single '-'. Non-ASCII is dropped rather
	// than transliterated: this runs once per legacy token, and a name with
	// nothing ASCII left falls back to TokenUsernameFallbackPrefix.
	var b strings.Builder

	pendingSep := false

	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_':
			if pendingSep && b.Len() > 0 {
				b.WriteByte('-')
			}

			pendingSep = false

			b.WriteRune(r)
		default:
			pendingSep = true
		}
	}

	// Truncate before trimming, so a cut that lands on a separator is
	// trimmed too. Everything written is ASCII, so byte slicing is safe.
	out := b.String()
	if len(out) > MaxTokenNamePrefixLen {
		out = out[:MaxTokenNamePrefixLen]
	}

	return strings.Trim(out, "-._")
}
