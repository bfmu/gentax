package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bmunoz/gentax/internal/auth"
)

// nextHandler is a simple handler that records whether it was called and
// copies the *Claims from context into the response header for assertions.
func nextHandler(t *testing.T, called *bool) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		claims := auth.ClaimsFromContext(r.Context())
		if claims != nil {
			w.Header().Set("X-User-ID", claims.UserID.String())
			w.Header().Set("X-Role", string(claims.Role))
		}
		w.WriteHeader(http.StatusOK)
	})
}

func issueToken(t *testing.T, svc *auth.JWTService, role auth.Role, ttl time.Duration) string {
	t.Helper()
	c := auth.Claims{
		UserID:  uuid.New(),
		Role:    role,
		OwnerID: uuid.New(),
	}
	tok, err := svc.Issue(c, ttl)
	require.NoError(t, err)
	return tok
}

// TestRequireAuth_ValidToken ensures a valid token passes through and injects claims.
func TestRequireAuth_ValidToken(t *testing.T) {
	svc := auth.NewJWTService("middleware-test-secret")
	token := issueToken(t, svc, auth.RoleAdmin, time.Hour)

	called := false
	mw := auth.RequireAuth(svc)
	handler := mw(nextHandler(t, &called))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called, "next handler should have been called")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("X-User-ID"))
	assert.Equal(t, "admin", rec.Header().Get("X-Role"))
}

// TestRequireAuth_MissingToken ensures a request without Authorization header gets 401.
func TestRequireAuth_MissingToken(t *testing.T) {
	svc := auth.NewJWTService("middleware-test-secret")

	called := false
	mw := auth.RequireAuth(svc)
	handler := mw(nextHandler(t, &called))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.False(t, called, "next handler should NOT have been called")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestRequireAuth_ExpiredToken ensures an expired token returns 401.
func TestRequireAuth_ExpiredToken(t *testing.T) {
	svc := auth.NewJWTService("middleware-test-secret")
	token := issueToken(t, svc, auth.RoleDriver, -time.Second)

	called := false
	mw := auth.RequireAuth(svc)
	handler := mw(nextHandler(t, &called))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.False(t, called, "next handler should NOT have been called")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestRequireAuth_NoAuthPrefix ensures "Token xxx" (no Bearer prefix) returns 401.
func TestRequireAuth_NoAuthPrefix(t *testing.T) {
	svc := auth.NewJWTService("middleware-test-secret")
	token := issueToken(t, svc, auth.RoleAdmin, time.Hour)

	called := false
	mw := auth.RequireAuth(svc)
	handler := mw(nextHandler(t, &called))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Token "+token) // wrong prefix
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestRequireAuth_BearerNoToken ensures "Bearer " with empty token returns 401.
func TestRequireAuth_BearerNoToken(t *testing.T) {
	svc := auth.NewJWTService("middleware-test-secret")

	called := false
	mw := auth.RequireAuth(svc)
	handler := mw(nextHandler(t, &called))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ") // empty token
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestClaimsFromContext_NilWhenMissing ensures ClaimsFromContext returns nil for empty context.
func TestClaimsFromContext_NilWhenMissing(t *testing.T) {
	ctx := t.Context()
	claims := auth.ClaimsFromContext(ctx)
	assert.Nil(t, claims)
}

// TestRequireAuth_AnyRoleAllowed ensures no role filter allows any valid token through.
func TestRequireAuth_AnyRoleAllowed(t *testing.T) {
	svc := auth.NewJWTService("middleware-test-secret")
	token := issueToken(t, svc, auth.RoleDriver, time.Hour)

	called := false
	mw := auth.RequireAuth(svc) // no role filter
	handler := mw(nextHandler(t, &called))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestRequireAuth_WrongRole ensures a token with a disallowed role returns 403.
func TestRequireAuth_WrongRole(t *testing.T) {
	svc := auth.NewJWTService("middleware-test-secret")
	// Issue a driver token but require admin.
	token := issueToken(t, svc, auth.RoleDriver, time.Hour)

	called := false
	mw := auth.RequireAuth(svc, auth.RoleAdmin) // only admin allowed
	handler := mw(nextHandler(t, &called))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.False(t, called, "next handler should NOT have been called")
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
