package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestSanitizeTokenNamePrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "valid prefix is unchanged", input: "billing.ci_2", want: "billing.ci_2"},
		{name: "valid prefix with dash is unchanged", input: "ci-deploy", want: "ci-deploy"},
		{name: "spaces and uppercase", input: "CI Pipeline", want: "ci-pipeline"},
		{name: "colon becomes separator", input: "billing:prod", want: "billing-prod"},
		{name: "separator runs collapse", input: "a  :  /b", want: "a-b"},
		{name: "edges are trimmed", input: "  -ci- ", want: "ci"},
		{name: "cyrillic only yields empty", input: "Деплой", want: ""},
		{name: "mixed cyrillic keeps ascii parts", input: "Деплой prod", want: "prod"},
		{name: "empty", input: "", want: ""},
		{
			name:  "truncation does not leave a trailing separator",
			input: strings.Repeat("a", domain.MaxTokenNamePrefixLen-1) + " bbb",
			want:  strings.Repeat("a", domain.MaxTokenNamePrefixLen-1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := domain.SanitizeTokenNamePrefix(tt.input)
			assert.Equal(t, tt.want, got)

			if got != "" {
				require.NoError(t, domain.ValidateTokenNamePrefix(got))
			}
		})
	}
}
