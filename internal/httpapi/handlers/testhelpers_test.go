package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bmunoz/gentax/internal/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// matchAny is a convenience alias for mock.Anything.
var matchAny = mock.Anything

// newAuthRequest builds an *http.Request with the given JWT claims injected into context.
func newAuthRequest(method, target string, claims *auth.Claims) *http.Request {
	r, _ := http.NewRequest(method, target, nil)
	if claims != nil {
		ctx := auth.ContextWithClaims(r.Context(), claims)
		r = r.WithContext(ctx)
	}
	return r
}

// adminClaims returns a minimal admin Claims with a random ownerID.
func adminClaims() (*auth.Claims, uuid.UUID) {
	ownerID := uuid.New()
	return &auth.Claims{
		UserID:  ownerID,
		Role:    auth.RoleAdmin,
		OwnerID: ownerID,
	}, ownerID
}

// driverClaims returns a minimal driver Claims with random ownerID and driverID.
func driverClaims() (*auth.Claims, uuid.UUID, uuid.UUID) {
	ownerID := uuid.New()
	driverID := uuid.New()
	return &auth.Claims{
		UserID:   driverID,
		Role:     auth.RoleDriver,
		OwnerID:  ownerID,
		DriverID: &driverID,
	}, ownerID, driverID
}

// nopCloser wraps an io.Reader with a no-op Close method so it can be assigned to
// http.Request.Body.
func nopCloser(r io.Reader) io.ReadCloser {
	return io.NopCloser(r)
}

// jsonBody returns an io.ReadCloser containing the JSON representation of v.
func jsonBody(v any) io.ReadCloser {
	b, _ := json.Marshal(v)
	return io.NopCloser(strings.NewReader(string(b)))
}

// assertErrorCode decodes the JSON body of w and asserts that the "code" field equals want.
func assertErrorCode(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body map[string]string
	assert.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, want, body["code"])
}
