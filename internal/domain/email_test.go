package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestNormalizeEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "plain lowercase", in: "alice@example.com", want: "alice@example.com"},
		{name: "uppercase folded", in: "Alice@Example.COM", want: "alice@example.com"},
		{name: "surrounding whitespace trimmed", in: "  bob@example.com\n", want: "bob@example.com"},
		{
			name: "fullwidth Latin folded via NFKC",
			// Fullwidth Latin "ＡＤＭＩＮ@example.com" → "admin@example.com"
			in:   "ＡＤＭＩＮ@example.com",
			want: "admin@example.com",
		},
		{
			name: "ligature folded via NFKC",
			// "alﬁ@example.com" (ﬁ ligature U+FB01) → "alfi@example.com"
			in:   "alﬁ@example.com",
			want: "alfi@example.com",
		},
		{name: "empty rejected", in: "", wantErr: true},
		{name: "whitespace-only rejected", in: "   ", wantErr: true},
		{name: "no @ rejected", in: "alice.example.com", wantErr: true},
		{name: "two @ rejected", in: "alice@two@example.com", wantErr: true},
		{name: "empty local part rejected", in: "@example.com", wantErr: true},
		{name: "empty domain rejected", in: "alice@", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.NormalizeEmail(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, domain.IsValidationError(err))

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestNormalizeEmail_Idempotent(t *testing.T) {
	t.Parallel()

	// NFKC normalization is idempotent — feeding the result back through
	// must produce the same string. This is what makes it safe to compare
	// already-normalized emails by raw equality at lookup time.
	cases := []string{"alice@example.com", "user+tag@example.co.uk", "x@y.io"}
	for _, in := range cases {
		first, err := domain.NormalizeEmail(in)
		require.NoError(t, err)
		second, err := domain.NormalizeEmail(first)
		require.NoError(t, err)
		assert.Equal(t, first, second, "NormalizeEmail must be idempotent")
	}
}
