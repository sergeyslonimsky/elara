package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/auth"
)

func TestWithClaims_And_ClaimsFromContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		claims *auth.Claims
	}{
		{
			name:   "with claims",
			claims: &auth.Claims{Email: "alice@example.com", Name: "Alice"},
		},
		{
			name:   "nil claims",
			claims: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := auth.WithClaims(context.Background(), tc.claims)
			got, ok := auth.ClaimsFromContext(ctx)

			require.True(t, ok)
			assert.Equal(t, tc.claims, got)
		})
	}
}

func TestClaimsFromContext_Missing(t *testing.T) {
	t.Parallel()

	got, ok := auth.ClaimsFromContext(context.Background())

	assert.False(t, ok)
	assert.Nil(t, got)
}
