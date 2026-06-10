package domain

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// RFC 5321 §4.5.3.1.3: SMTP path including angle brackets, conservative upper bound on the address itself.
const maxEmailLen = 254

// NormalizeEmail produces the canonical form used as both User.Email storage
// value and the lookup key for the users_by_email index. It performs:
//
//   - Whitespace trim,
//   - Unicode NFKC (folds compatibility-equivalent codepoints — e.g. "fi"
//     ligature → "fi", fullwidth Latin → ASCII Latin),
//   - Lowercase (case-insensitive matching; technically the local-part of an
//     email is case-sensitive per RFC 5321 but every mainstream IdP treats it
//     case-insensitively, so we collapse it),
//   - Exactly-one-`@` shape check,
//   - Non-empty local and domain halves,
//   - Length cap (254 chars).
//
// Returns the normalized form and a ValidationError on shape violations.
//
// This is the SINGLE place email normalization happens — every callsite that
// stores or queries User.Email must go through it so the index never sees
// pre-NFKC or differently-cased forms of the same address.
func NormalizeEmail(s string) (string, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "", NewValidationError("email", "email is required")
	}

	normalized := norm.NFKC.String(trimmed)
	normalized = strings.ToLower(normalized)

	if len(normalized) > maxEmailLen {
		return "", NewValidationError("email", "email exceeds maximum length")
	}

	at := strings.IndexByte(normalized, '@')
	if at <= 0 || at == len(normalized)-1 || strings.IndexByte(normalized[at+1:], '@') != -1 {
		return "", NewValidationError("email", "email must contain exactly one @ with non-empty local and domain parts")
	}

	return normalized, nil
}
