package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

var errUnexpectedSigningMethod = errors.New("unexpected signing method")

// Claims holds the user identity extracted from a JWT session token.
type Claims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

// SessionManager creates and validates HS256 JWT session tokens.
type SessionManager struct {
	secret []byte
	ttl    time.Duration
}

// NewSessionManager returns a SessionManager using the given HMAC secret and token TTL.
func NewSessionManager(secret string, ttl time.Duration) *SessionManager {
	return &SessionManager{secret: []byte(secret), ttl: ttl}
}

// Create signs a new JWT for the given user.
func (m *SessionManager) Create(user *domain.User) (string, error) {
	now := time.Now()
	claims := Claims{
		Email: user.Email,
		Name:  user.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return signed, nil
}

// Validate parses and verifies the token, returning Claims on success.
// Returns domain.ErrInvalidToken if the token is malformed, expired, or signed with a wrong secret.
func (m *SessionManager) Validate(tokenStr string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: %v", errUnexpectedSigningMethod, t.Header["alg"])
		}

		return m.secret, nil
	}, jwt.WithExpirationRequired())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrInvalidToken, err)
	}

	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, domain.ErrInvalidToken
	}

	return claims, nil
}
