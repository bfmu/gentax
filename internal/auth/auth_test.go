package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bmunoz/gentax/internal/auth"
)

func newService(t *testing.T) *auth.JWTService {
	t.Helper()
	return auth.NewJWTService("super-secret-key-for-testing")
}

// TestTokenIssuer_SignAndValidate issues a token and validates it, asserting claims match.
func TestTokenIssuer_SignAndValidate(t *testing.T) {
	svc := newService(t)

	driverID := uuid.New()
	ownerID := uuid.New()
	userID := uuid.New()

	original := auth.Claims{
		UserID:  userID,
		Role:    auth.RoleDriver,
		OwnerID: ownerID,
		DriverID: &driverID,
	}

	token, err := svc.Issue(original, time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	got, err := svc.Validate(token)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, original.UserID, got.UserID)
	assert.Equal(t, original.Role, got.Role)
	assert.Equal(t, original.OwnerID, got.OwnerID)
	require.NotNil(t, got.DriverID)
	assert.Equal(t, *original.DriverID, *got.DriverID)
}

// TestTokenIssuer_ExpiredToken issues a token with 0 duration, then validates it and asserts expiry error.
func TestTokenIssuer_ExpiredToken(t *testing.T) {
	svc := newService(t)

	claims := auth.Claims{
		UserID:  uuid.New(),
		Role:    auth.RoleAdmin,
		OwnerID: uuid.New(),
	}

	// Issue with -1s TTL so it's immediately expired.
	token, err := svc.Issue(claims, -time.Second)
	require.NoError(t, err)

	_, err = svc.Validate(token)
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrTokenExpired)
}

// TestTokenIssuer_TamperedToken issues a token, modifies one byte, validates, asserts error.
func TestTokenIssuer_TamperedToken(t *testing.T) {
	svc := newService(t)

	claims := auth.Claims{
		UserID:  uuid.New(),
		Role:    auth.RoleAdmin,
		OwnerID: uuid.New(),
	}

	token, err := svc.Issue(claims, time.Hour)
	require.NoError(t, err)

	// Tamper: flip the last byte.
	tampered := []byte(token)
	tampered[len(tampered)-1] ^= 0xFF
	tamperedStr := string(tampered)

	_, err = svc.Validate(tamperedStr)
	require.Error(t, err)
}

// TestTokenIssuer_WrongAlgorithm creates a HS512-signed token with the same secret and asserts
// that Validate rejects it due to algorithm mismatch.
func TestTokenIssuer_WrongAlgorithm(t *testing.T) {
	secret := []byte("super-secret-key-for-testing")

	jwtClaims := jwt.MapClaims{
		"user_id":  uuid.New().String(),
		"role":     "admin",
		"owner_id": uuid.New().String(),
		"exp":      time.Now().Add(time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	}

	// Sign with HS512 — but our service only accepts HS256.
	wrongAlgToken := jwt.NewWithClaims(jwt.SigningMethodHS512, jwtClaims)
	tokenStr, err := wrongAlgToken.SignedString(secret)
	require.NoError(t, err)

	svc := auth.NewJWTService("super-secret-key-for-testing")
	_, err = svc.Validate(tokenStr)
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}

// TestTokenIssuer_AdminNoDriverID asserts nil DriverID for admin role round-trips correctly.
func TestTokenIssuer_AdminNoDriverID(t *testing.T) {
	svc := newService(t)

	original := auth.Claims{
		UserID:   uuid.New(),
		Role:     auth.RoleAdmin,
		OwnerID:  uuid.New(),
		DriverID: nil, // admin has no driver ID
	}

	token, err := svc.Issue(original, time.Hour)
	require.NoError(t, err)

	got, err := svc.Validate(token)
	require.NoError(t, err)

	assert.Nil(t, got.DriverID)
	assert.Equal(t, auth.RoleAdmin, got.Role)
}

// TestTokenIssuer_MalformedToken checks a completely invalid token string.
func TestTokenIssuer_MalformedToken(t *testing.T) {
	svc := newService(t)
	_, err := svc.Validate("not.a.jwt")
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}

// TestTokenIssuer_ClaimsRoundtrip asserts all claims survive sign→validate.
func TestTokenIssuer_ClaimsRoundtrip(t *testing.T) {
	svc := newService(t)

	driverID := uuid.New()
	original := auth.Claims{
		UserID:   uuid.New(),
		Role:     auth.RoleDriver,
		OwnerID:  uuid.New(),
		DriverID: &driverID,
	}

	token, err := svc.Issue(original, 2*time.Hour)
	require.NoError(t, err)

	got, err := svc.Validate(token)
	require.NoError(t, err)

	assert.Equal(t, original.UserID, got.UserID)
	assert.Equal(t, original.Role, got.Role)
	assert.Equal(t, original.OwnerID, got.OwnerID)
	require.NotNil(t, got.DriverID)
	assert.Equal(t, *original.DriverID, *got.DriverID)
	// RegisteredClaims should be populated.
	assert.False(t, got.IssuedAt.IsZero())
	assert.False(t, got.ExpiresAt.IsZero())
}

// TestContextWithClaims_InjectsAndExtractsClaims verifies that ContextWithClaims
// stores claims in the context and ClaimsFromContext retrieves them correctly.
func TestContextWithClaims_InjectsAndExtractsClaims(t *testing.T) {
	driverID := uuid.New()
	original := &auth.Claims{
		UserID:   uuid.New(),
		Role:     auth.RoleDriver,
		OwnerID:  uuid.New(),
		DriverID: &driverID,
	}

	ctx := auth.ContextWithClaims(t.Context(), original)
	got := auth.ClaimsFromContext(ctx)

	require.NotNil(t, got)
	assert.Equal(t, original.UserID, got.UserID)
	assert.Equal(t, original.Role, got.Role)
	assert.Equal(t, original.OwnerID, got.OwnerID)
	require.NotNil(t, got.DriverID)
	assert.Equal(t, *original.DriverID, *got.DriverID)
}

// TestContextWithClaims_NilClaimsRoundtrips verifies that a nil *Claims value
// injected via ContextWithClaims is returned as nil by ClaimsFromContext,
// consistent with unauthenticated-route semantics.
func TestContextWithClaims_NilClaimsRoundtrips(t *testing.T) {
	ctx := auth.ContextWithClaims(t.Context(), nil)
	got := auth.ClaimsFromContext(ctx)
	assert.Nil(t, got)
}
