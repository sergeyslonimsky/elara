package auth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestSessionManager_Create_And_Validate(t *testing.T) {
	t.Parallel()

	mgr := auth.NewSessionManager("super-secret", time.Hour)
	user := &domain.User{Email: "alice@example.com", Name: "Alice"}

	token, err := mgr.Create(user)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := mgr.Validate(token)
	require.NoError(t, err)
	assert.Equal(t, user.Email, claims.Email)
	assert.Equal(t, user.Name, claims.Name)
}

func TestSessionManager_Validate_WrongSecret(t *testing.T) {
	t.Parallel()

	creator := auth.NewSessionManager("correct-secret", time.Hour)
	validator := auth.NewSessionManager("wrong-secret", time.Hour)

	user := &domain.User{Email: "alice@example.com", Name: "Alice"}
	token, err := creator.Create(user)
	require.NoError(t, err)

	_, err = validator.Validate(token)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidToken)
}

func TestSessionManager_Validate_Expired(t *testing.T) {
	t.Parallel()

	mgr := auth.NewSessionManager("super-secret", -time.Second)

	user := &domain.User{Email: "alice@example.com", Name: "Alice"}
	token, err := mgr.Create(user)
	require.NoError(t, err)

	_, err = mgr.Validate(token)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidToken)
}

func TestSessionManager_Validate_EmptyString(t *testing.T) {
	t.Parallel()

	mgr := auth.NewSessionManager("super-secret", time.Hour)

	_, err := mgr.Validate("")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidToken)
}

func TestSessionManager_Validate_Malformed(t *testing.T) {
	t.Parallel()

	mgr := auth.NewSessionManager("super-secret", time.Hour)

	_, err := mgr.Validate("not.a.valid.jwt")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidToken)
}
