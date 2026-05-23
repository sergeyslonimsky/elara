package authz_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/service/authz"
)

func TestNewDomainSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []string
		wantWild bool
		wantExpl []string
	}{
		{
			name:     "explicit domains",
			input:    []string{"dev", "prod"},
			wantWild: false,
			wantExpl: []string{"dev", "prod"},
		},
		{
			name:     "wildcard",
			input:    []string{"dev", "*", "prod"},
			wantWild: true,
			wantExpl: []string{"dev", "prod"},
		},
		{
			name:     "empty",
			input:    []string{},
			wantWild: false,
			wantExpl: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := authz.NewDomainSet(tt.input...)

			assert.Equal(t, tt.wantWild, got.Wildcard)
			require.Len(t, got.Explicit, len(tt.wantExpl))
			for _, d := range tt.wantExpl {
				assert.Contains(t, got.Explicit, d)
			}
		})
	}
}

func TestDomainSet_Contains(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		set    authz.DomainSet
		domain string
		want   bool
	}{
		{
			name:   "wildcard true returns true for any domain",
			set:    authz.NewDomainSet("*"),
			domain: "any",
			want:   true,
		},
		{
			name:   "wildcard true returns true for empty domain",
			set:    authz.NewDomainSet("*"),
			domain: "",
			want:   true,
		},
		{
			name:   "wildcard false returns true if domain is in explicit",
			set:    authz.NewDomainSet("dev", "prod"),
			domain: "dev",
			want:   true,
		},
		{
			name:   "wildcard false returns false if domain is not in explicit",
			set:    authz.NewDomainSet("dev", "prod"),
			domain: "stg",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.set.Contains(tt.domain)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDomainSet_IsEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		set  authz.DomainSet
		want bool
	}{
		{
			name: "wildcard true returns false",
			set:  authz.NewDomainSet("*"),
			want: false,
		},
		{
			name: "wildcard false explicit empty returns true",
			set:  authz.NewDomainSet(),
			want: true,
		},
		{
			name: "wildcard false explicit not empty returns false",
			set:  authz.NewDomainSet("dev"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.set.IsEmpty()
			assert.Equal(t, tt.want, got)
		})
	}
}
