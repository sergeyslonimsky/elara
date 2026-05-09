package bbolt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestAuthUserMetaRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	u := &domain.User{
		Email:                  "user@example.com",
		Name:                   "User",
		Picture:                "pic",
		Provider:               "oidc",
		CreatedAt:              now,
		LastLoginAt:            now,
		PasswordHash:           "hash",
		PasswordChangeRequired: true,
	}

	meta := domainToAuthUserMeta(u)
	got := authUserMetaToDomain(meta)

	assert.Equal(t, u, got)
}

func TestAuthGroupMetaRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	g := &domain.Group{
		ID:        "group-1",
		Name:      "Group 1",
		Members:   []string{"a@example.com", "b@example.com"},
		CreatedAt: now,
		UpdatedAt: now,
	}

	meta := domainToAuthGroupMeta(g)
	got := authGroupMetaToDomain(meta)

	assert.Equal(t, g, got)
	// Verify deep copy of Members
	meta.Members[0] = "changed"
	assert.Equal(t, "a@example.com", g.Members[0])
}

func TestAuthTokenMetaRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	tkn := &domain.Token{
		ID:         "token-1",
		IssuedBy:   "user@example.com",
		Name:       "Token 1",
		TokenHash:  "hash",
		Namespaces: []string{"ns1", "ns2"},
		Role:       "writer",
		ExpiresAt:  new(now.Add(time.Hour)),
		LastUsedAt: new(now.Add(time.Minute)),
		LastUsedIP: "1.2.3.4",
		CreatedAt:  now,
	}

	meta := domainToAuthTokenMeta(tkn)
	got := authTokenMetaToDomain(meta)

	assert.Equal(t, tkn, got)
	// Verify deep copy of Namespaces
	meta.Namespaces[0] = "changed"
	assert.Equal(t, "ns1", tkn.Namespaces[0])
}

func TestAuthTokenMetaFromBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    []byte
		wantID  string
		wantErr string
	}{
		{
			name:   "valid JSON",
			data:   []byte(`{"id": "t1", "name": "token"}`),
			wantID: "t1",
		},
		{
			name:    "invalid JSON",
			data:    []byte(`{invalid}`),
			wantErr: "unmarshal token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			meta, err := authTokenMetaFromBytes(tt.data)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, meta)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantID, meta.ID)
		})
	}
}

func TestAuthGroupMetaFromBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    []byte
		wantID  string
		wantErr string
	}{
		{
			name:   "valid JSON",
			data:   []byte(`{"id": "g1", "name": "group"}`),
			wantID: "g1",
		},
		{
			name:    "invalid JSON",
			data:    []byte(`{invalid}`),
			wantErr: "unmarshal group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			meta, err := authGroupMetaFromBytes(tt.data)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, meta)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantID, meta.ID)
		})
	}
}
