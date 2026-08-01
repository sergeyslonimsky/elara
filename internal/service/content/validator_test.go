package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/content"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		format  domain.Format
		wantErr string
	}{
		{
			name:    "valid json",
			content: `{"a":1}`,
			format:  domain.FormatJSON,
		},
		{
			name:    "invalid json",
			content: `{"a":`,
			format:  domain.FormatJSON,
			wantErr: "unmarshal JSON:",
		},
		{
			name:    "valid yaml",
			content: "a: 1\n",
			format:  domain.FormatYAML,
		},
		{
			name:    "invalid yaml",
			content: "a: [1, 2\n",
			format:  domain.FormatYAML,
			wantErr: "unmarshal YAML:",
		},
		{
			name:    "other format always valid",
			content: "anything goes here",
			format:  domain.FormatOther,
		},
		{
			name:    "unknown format",
			content: `{"a":1}`,
			format:  domain.Format("xml"),
			wantErr: "validate content: invalid format: xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := content.Validate(tt.content, tt.format)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}

func TestDetectFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    domain.Format
		wantErr string
	}{
		{
			name:    "detects json",
			content: `{"a":1}`,
			want:    domain.FormatJSON,
		},
		{
			name:    "detects yaml",
			content: "a: 1\nb: 2\n",
			want:    domain.FormatYAML,
		},
		{
			name:    "empty content after trim",
			content: "   \n\t  ",
			wantErr: "validation: content: empty content",
		},
		{
			name:    "neither json nor yaml",
			content: "a: [1, 2\n",
			wantErr: "validation: content: content is neither valid JSON nor YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := content.DetectFormat(tt.content)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		format  domain.Format
		want    string
		wantErr string
		errIs   error
	}{
		{
			name:    "normalizes json",
			content: `{"b":2,"a":1}`,
			format:  domain.FormatJSON,
			want:    "{\n  \"a\": 1,\n  \"b\": 2\n}",
		},
		{
			name:    "invalid json returns wrapped ErrInvalidContent",
			content: `{"a":`,
			format:  domain.FormatJSON,
			errIs:   domain.ErrInvalidContent,
			wantErr: "unmarshal JSON:",
		},
		{
			name:    "normalizes yaml",
			content: "b: 2\na: 1\n",
			format:  domain.FormatYAML,
			want:    "a: 1\nb: 2\n",
		},
		{
			name:    "invalid yaml returns wrapped ErrInvalidContent",
			content: "a: [1, 2\n",
			format:  domain.FormatYAML,
			errIs:   domain.ErrInvalidContent,
			wantErr: "unmarshal YAML:",
		},
		{
			name:    "other format passes through unchanged",
			content: "raw content",
			format:  domain.FormatOther,
			want:    "raw content",
		},
		{
			name:    "unknown format passes through unchanged",
			content: "raw content",
			format:  domain.Format("xml"),
			want:    "raw content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := content.Normalize(tt.content, tt.format)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateAndNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		format  domain.Format
		want    *domain.ValidationResult
	}{
		{
			name:    "valid json with explicit format",
			content: `{"b":2,"a":1}`,
			format:  domain.FormatJSON,
			want: &domain.ValidationResult{
				Valid:             true,
				DetectedFormat:    domain.FormatJSON,
				NormalizedContent: "{\n  \"a\": 1,\n  \"b\": 2\n}",
			},
		},
		{
			name:    "valid yaml with format auto-detected",
			content: "b: 2\na: 1\n",
			want: &domain.ValidationResult{
				Valid:             true,
				DetectedFormat:    domain.FormatYAML,
				NormalizedContent: "a: 1\nb: 2\n",
			},
		},
		{
			name:    "detect failure produces result with errors, no top-level error",
			content: "   ",
			want: &domain.ValidationResult{
				Valid:  false,
				Errors: []string{"validation: content: empty content"},
			},
		},
		{
			name:    "explicit format validate failure produces result with errors",
			content: `{"a":`,
			format:  domain.FormatJSON,
			want: &domain.ValidationResult{
				Valid:          false,
				DetectedFormat: domain.FormatJSON,
				Errors:         []string{"unmarshal JSON: unexpected end of JSON input"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := content.ValidateAndNormalize(tt.content, tt.format)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateAndNormalize_NormalizeFailureIsUnreachableViaPublicAPI(t *testing.T) {
	t.Parallel()

	// Normalize can only fail after Validate already succeeded for the same
	// content/format pair, since both parse the same input the same way.
	// This test documents that ValidateAndNormalize never surfaces a
	// top-level (non-nil) error for any input — invalid content is always
	// reported via ValidationResult.Errors.
	_, err := content.ValidateAndNormalize("not json or yaml at all: [", domain.FormatJSON)

	require.NoError(t, err)
}

func TestValidate_UnknownFormatWrapsErrInvalidFormat(t *testing.T) {
	t.Parallel()

	err := content.Validate("irrelevant", domain.Format("bogus"))

	require.ErrorIs(t, err, domain.ErrInvalidFormat)
}
