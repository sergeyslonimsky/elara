package auth_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func TestHashPassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		plain string
	}{
		{name: "regular password", plain: "s3cr3tP@ssword"},
		{name: "empty password", plain: ""},
		{name: "unicode password", plain: "пароль123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hash1, err := auth.HashPassword(tt.plain)
			require.NoError(t, err)
			assert.NotEmpty(t, hash1)

			// bcrypt adds a random salt — two calls must produce different hashes.
			hash2, err := auth.HashPassword(tt.plain)
			require.NoError(t, err)
			assert.NotEqual(t, hash1, hash2, "salt not working")

			// The produced hash must verify correctly.
			err = auth.VerifyPassword(hash1, tt.plain)
			assert.NoError(t, err)
		})
	}
}

func TestHashPassword_TooLongFails(t *testing.T) {
	t.Parallel()

	// bcrypt rejects inputs over 72 bytes.
	tooLong := strings.Repeat("a", 73)

	_, err := auth.HashPassword(tooLong)
	require.ErrorContains(t, err, "hash password")
}

func TestVerifyPassword(t *testing.T) {
	t.Parallel()

	const correctPlain = "correct-horse-battery-staple"
	correctHash, err := auth.HashPassword(correctPlain)
	require.NoError(t, err)

	tests := []struct {
		name    string
		hash    string
		plain   string
		wantErr string
	}{
		{
			name:  "correct password",
			hash:  correctHash,
			plain: correctPlain,
		},
		{
			name:    "wrong password",
			hash:    correctHash,
			plain:   "wrong-password",
			wantErr: "crypto/bcrypt: hashedPassword is not the hash of the given password",
		},
		{
			name:    "empty plain against valid hash",
			hash:    correctHash,
			plain:   "",
			wantErr: "crypto/bcrypt: hashedPassword is not the hash of the given password",
		},
		{
			name:    "empty hash",
			hash:    "",
			plain:   correctPlain,
			wantErr: "crypto/bcrypt: hashedSecret too short to be a bcrypted password",
		},
		{
			name:    "malformed hash",
			hash:    "not-a-bcrypt-hash",
			plain:   correctPlain,
			wantErr: "crypto/bcrypt: hashedSecret too short to be a bcrypted password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := auth.VerifyPassword(tt.hash, tt.plain)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}
