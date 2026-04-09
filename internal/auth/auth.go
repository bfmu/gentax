// Package auth provides JWT issuance, validation, and Chi middleware for gentax.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Sentinel errors returned by Validate.
var (
	// ErrTokenExpired is returned when the token's exp claim is in the past.
	ErrTokenExpired = errors.New("token expired")
	// ErrInvalidToken is returned for any other validation failure (bad signature,
	// wrong algorithm, malformed token, etc.).
	ErrInvalidToken = errors.New("invalid token")
)

// Role controls access level within the system.
type Role string

const (
	RoleDriver Role = "driver"
	RoleAdmin  Role = "admin"
)

// Claims are the JWT payload fields injected into every authenticated request context.
type Claims struct {
	UserID   uuid.UUID  `json:"user_id"`
	Role     Role       `json:"role"`
	OwnerID  uuid.UUID  `json:"owner_id"`
	DriverID *uuid.UUID `json:"driver_id,omitempty"` // nil for admin
	jwt.RegisteredClaims
}

// contextKey is the unexported key for storing *Claims in context.Context.
type contextKey struct{}

// claimsKey is the singleton key used by middleware and ClaimsFromContext.
var claimsKey = contextKey{}

// TokenIssuer creates signed JWTs.
type TokenIssuer interface {
	// Issue signs a new JWT for the given claims with the provided TTL.
	Issue(claims Claims, ttl time.Duration) (string, error)
}

// TokenValidator parses and validates JWT strings.
type TokenValidator interface {
	// Validate parses the token, checks signature and expiry, returns claims.
	Validate(token string) (*Claims, error)
}

// JWTService implements both TokenIssuer and TokenValidator using HS256.
type JWTService struct {
	secret []byte
}

// NewJWTService creates a JWTService with the provided signing secret.
func NewJWTService(secret string) *JWTService {
	return &JWTService{secret: []byte(secret)}
}

// Issue signs a new JWT containing the given claims. The token expires after ttl.
// Algorithm: HS256.
func (s *JWTService) Issue(claims Claims, ttl time.Duration) (string, error) {
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// Validate parses and validates a JWT string. It rejects tokens that are not
// signed with HS256, have an invalid signature, or are expired.
func (s *JWTService) Validate(tokenStr string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		// Explicitly reject any algorithm that is not HS256.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method %v", ErrInvalidToken, t.Header["alg"])
		}
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("%w: expected HS256, got %v", ErrInvalidToken, t.Header["alg"])
		}
		return s.secret, nil
	})

	if err != nil {
		// Map jwt library errors to our sentinels.
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		// Check for our own wrapped ErrInvalidToken from the key function.
		if errors.Is(err, ErrInvalidToken) {
			return nil, ErrInvalidToken
		}
		return nil, ErrInvalidToken
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
