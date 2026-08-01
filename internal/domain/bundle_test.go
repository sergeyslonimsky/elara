package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestImportReport_AddError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		initial    domain.ImportReport
		path       string
		namespace  string
		message    string
		wantFailed int
		wantErrors []domain.BundleImportError
	}{
		{
			name:       "add error to empty report",
			initial:    domain.ImportReport{},
			path:       "configs/a.yaml",
			namespace:  "prod",
			message:    "invalid format",
			wantFailed: 1,
			wantErrors: []domain.BundleImportError{
				{Path: "configs/a.yaml", Namespace: "prod", Message: "invalid format"},
			},
		},
		{
			name: "add error appends to existing errors and increments failed",
			initial: domain.ImportReport{
				Failed: 1,
				Errors: []domain.BundleImportError{
					{Path: "configs/x.yaml", Namespace: "dev", Message: "first error"},
				},
			},
			path:       "configs/b.yaml",
			namespace:  "staging",
			message:    "second error",
			wantFailed: 2,
			wantErrors: []domain.BundleImportError{
				{Path: "configs/x.yaml", Namespace: "dev", Message: "first error"},
				{Path: "configs/b.yaml", Namespace: "staging", Message: "second error"},
			},
		},
		{
			name:       "empty path namespace and message still recorded",
			initial:    domain.ImportReport{},
			path:       "",
			namespace:  "",
			message:    "",
			wantFailed: 1,
			wantErrors: []domain.BundleImportError{
				{Path: "", Namespace: "", Message: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			report := tt.initial
			report.AddError(tt.path, tt.namespace, tt.message)

			assert.Equal(t, tt.wantFailed, report.Failed)
			assert.Equal(t, tt.wantErrors, report.Errors)
		})
	}
}
