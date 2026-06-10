package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestValidationError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		field    string
		msg      string
		expected string
	}{
		{
			name:     "with field",
			field:    "name",
			msg:      "required",
			expected: "validation: name: required",
		},
		{
			name:     "without field",
			field:    "",
			msg:      "invalid",
			expected: "validation: invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ve := domain.NewValidationError(tt.field, tt.msg)
			assert.Equal(t, tt.expected, ve.Error())
		})
	}
}

func TestIsValidationError(t *testing.T) {
	t.Parallel()

	ve := domain.NewValidationError("field", "msg")
	assert.True(t, domain.IsValidationError(ve))
	assert.True(t, domain.IsValidationError(fmt.Errorf("wrap: %w", ve)))

	assert.False(t, domain.IsValidationError(errors.New("other")))
}

func TestErrorHelpers(t *testing.T) {
	t.Parallel()

	t.Run("LockedError", func(t *testing.T) {
		t.Parallel()

		err := domain.NewLockedError("path/to/config")
		require.ErrorIs(t, err, domain.ErrLocked)
		assert.Contains(t, err.Error(), "path/to/config")
	})

	t.Run("InvalidFormatError", func(t *testing.T) {
		t.Parallel()

		err := domain.NewInvalidFormatError("toml")
		require.ErrorIs(t, err, domain.ErrInvalidFormat)
		assert.Contains(t, err.Error(), "toml")
	})

	t.Run("NotFoundError", func(t *testing.T) {
		t.Parallel()

		err := domain.NewNotFoundError("User", "alice")
		require.ErrorIs(t, err, domain.ErrNotFound)
		assert.Contains(t, err.Error(), "User")
		assert.Contains(t, err.Error(), "alice")
	})

	t.Run("AlreadyExistsError", func(t *testing.T) {
		t.Parallel()

		err := domain.NewAlreadyExistsError("Group", "admins")
		require.ErrorIs(t, err, domain.ErrAlreadyExists)
		assert.Contains(t, err.Error(), "Group")
		assert.Contains(t, err.Error(), "admins")
	})

	t.Run("ConflictError", func(t *testing.T) {
		t.Parallel()

		err := domain.NewConflictError(10, 5)
		require.ErrorIs(t, err, domain.ErrVersionConflict)
		assert.Contains(t, err.Error(), "10")
		assert.Contains(t, err.Error(), "5")
	})
}

func TestCheckVersion(t *testing.T) {
	t.Parallel()

	var ten int64 = 10

	tests := []struct {
		name     string
		expected *int64
		current  int64
		errIs    error
	}{
		{
			name:     "nil expected passes",
			expected: nil,
			current:  5,
		},
		{
			name:     "matching values pass",
			expected: &ten,
			current:  10,
		},
		{
			name:     "mismatching values fail",
			expected: &ten,
			current:  5,
			errIs:    domain.ErrVersionConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := domain.CheckVersion(tt.expected, tt.current)
			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			require.NoError(t, err)
		})
	}
}
