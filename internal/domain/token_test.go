package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func validToken() domain.Token {
	return domain.Token{
		ID:         "token-1",
		IssuedBy:   "alice@example.com",
		Name:       "My Token",
		TokenHash:  "abc123def456",
		Namespaces: []string{"prod"},
		Role:       "reader",
		CreatedAt:  time.Now(),
	}
}

func TestToken_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		token   domain.Token
		wantErr bool
		errMsg  string
	}{
		{name: "valid token", token: validToken()},
		{
			name: "empty ID",
			token: func() domain.Token {
				tok := validToken()
				tok.ID = ""

				return tok
			}(),
			wantErr: true, errMsg: "id",
		},
		{
			name: "empty issuedBy",
			token: func() domain.Token {
				tok := validToken()
				tok.IssuedBy = ""

				return tok
			}(),
			wantErr: true, errMsg: "issuedBy",
		},
		{
			name: "invalid email no at sign",
			token: func() domain.Token {
				tok := validToken()
				tok.IssuedBy = "notanemail"

				return tok
			}(),
			wantErr: true, errMsg: "issuedBy",
		},
		{
			name: "empty name",
			token: func() domain.Token {
				tok := validToken()
				tok.Name = ""

				return tok
			}(),
			wantErr: true, errMsg: "name",
		},
		{
			name: "name too long",
			token: func() domain.Token {
				tok := validToken()
				tok.Name = strings.Repeat("a", 129)

				return tok
			}(),
			wantErr: true, errMsg: "name",
		},
		{
			name: "name at max length is valid",
			token: func() domain.Token {
				tok := validToken()
				tok.Name = strings.Repeat("a", 128)

				return tok
			}(),
		},
		{
			name: "empty token hash",
			token: func() domain.Token {
				tok := validToken()
				tok.TokenHash = ""

				return tok
			}(),
			wantErr: true, errMsg: "tokenHash",
		},
		{
			name: "invalid role",
			token: func() domain.Token {
				tok := validToken()
				tok.Role = "admin"

				return tok
			}(),
			wantErr: true, errMsg: "role",
		},
		{
			name: "writer role is valid",
			token: func() domain.Token {
				tok := validToken()
				tok.Role = "writer"

				return tok
			}(),
		},
		{
			name: "empty namespaces returns error",
			token: func() domain.Token {
				tok := validToken()
				tok.Namespaces = nil

				return tok
			}(),
			wantErr: true, errMsg: "namespaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.token.Validate()

			if tt.wantErr {
				require.Error(t, err)
				assert.True(t, domain.IsValidationError(err))

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestToken_IsExpired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		expiresAt *time.Time
		want      bool
	}{
		{name: "nil expiry never expires", want: false},
		{name: "past expiry is expired", expiresAt: new(time.Now().Add(-time.Hour)), want: true},
		{name: "future expiry is not expired", expiresAt: new(time.Now().Add(time.Hour)), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tok := validToken()
			tok.ExpiresAt = tt.expiresAt

			assert.Equal(t, tt.want, tok.IsExpired())
		})
	}
}

func TestToken_NamespaceAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		namespaces []string
		namespace  string
		want       bool
	}{
		{name: "empty namespaces denies access", namespaces: []string{}, namespace: "production", want: false},
		{name: "nil namespaces denies access", namespaces: nil, namespace: "production", want: false},
		{
			name:       "matching namespace grants access",
			namespaces: []string{"staging", "production"},
			namespace:  "production",
			want:       true,
		},
		{
			name:       "non-matching namespace denies access",
			namespaces: []string{"staging"},
			namespace:  "production",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tok := validToken()
			tok.Namespaces = tt.namespaces

			assert.Equal(t, tt.want, tok.NamespaceAllowed(tt.namespace))
		})
	}
}

func TestToken_ActionAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		role   domain.Role
		action domain.Action
		want   bool
	}{
		{name: "writer can read", role: domain.RoleWriter, action: domain.ActionRead, want: true},
		{name: "writer can write", role: domain.RoleWriter, action: domain.ActionWrite, want: true},
		{name: "reader can read", role: domain.RoleReader, action: domain.ActionRead, want: true},
		{name: "reader cannot write", role: domain.RoleReader, action: domain.ActionWrite, want: false},
		{name: "unknown role denied", role: "", action: domain.ActionRead, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tok := validToken()
			tok.Role = tt.role

			assert.Equal(t, tt.want, tok.ActionAllowed(tt.action))
		})
	}
}
