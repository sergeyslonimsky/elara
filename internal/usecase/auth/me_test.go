package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

// stubMeEnforcer is a simple meEnforcer stub for unit tests.
type stubMeEnforcer struct {
	results map[string]bool
}

func (s *stubMeEnforcer) Enforce(subject, ns, object, action string) (bool, error) {
	key := subject + ":" + ns + ":" + object + ":" + action

	return s.results[key], nil
}

// stubNsLister returns a fixed list of namespaces.
type stubNsLister struct {
	items []*domain.Namespace
}

func (s *stubNsLister) List(_ context.Context) ([]*domain.Namespace, error) {
	return s.items, nil
}

func TestMeUseCase_Execute_WithClaims(t *testing.T) {
	t.Parallel()

	enforcer := &stubMeEnforcer{results: map[string]bool{
		"user@example.com:prod:config:read": true,
	}}
	nsList := &stubNsLister{items: []*domain.Namespace{{Name: "prod"}}}

	uc := authuc.NewMeUseCase(enforcer, nsList)

	claims := &auth.Claims{Email: "user@example.com", Name: "Test User"}
	ctx := auth.WithClaims(t.Context(), claims)

	result, err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.Equal(t, "user@example.com", result.Email)
	assert.Equal(t, "Test User", result.Name)
	assert.Len(t, result.Namespaces, 1)
	assert.Equal(t, "prod", result.Namespaces[0].Name)
}

func TestMeUseCase_Execute_NoClaims(t *testing.T) {
	t.Parallel()

	enforcer := &stubMeEnforcer{results: map[string]bool{}}
	nsList := &stubNsLister{items: nil}

	uc := authuc.NewMeUseCase(enforcer, nsList)

	_, err := uc.Execute(t.Context())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}
