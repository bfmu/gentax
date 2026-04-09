package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bmunoz/gentax/internal/auth"
	"github.com/bmunoz/gentax/internal/owner"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockOwnerService is a testify mock for OwnerAuthService.
type mockOwnerService struct{ mock.Mock }

func (m *mockOwnerService) Authenticate(ctx context.Context, email, password string) (*owner.Owner, error) {
	args := m.Called(ctx, email, password)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*owner.Owner), args.Error(1)
}

func (m *mockOwnerService) Create(ctx context.Context, name, email, password string) (*owner.Owner, error) {
	args := m.Called(ctx, name, email, password)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*owner.Owner), args.Error(1)
}

func (m *mockOwnerService) Count(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

// --- OwnerLogin ---

func TestOwnerLogin_Success(t *testing.T) {
	ownerSvc := new(mockOwnerService)
	issuer := new(mockTokenIssuer)
	h := NewOwnerAuthHandler(ownerSvc, issuer, "")

	ownerID := uuid.New()
	o := &owner.Owner{ID: ownerID, Name: "Boss", Email: "boss@example.com"}

	ownerSvc.On("Authenticate", matchAny, "boss@example.com", "pass123").Return(o, nil)
	issuer.On("Issue", auth.Claims{
		UserID:  ownerID,
		Role:    auth.RoleAdmin,
		OwnerID: ownerID,
	}, 8*time.Hour).Return("owner.jwt.token", nil)

	body := `{"email":"boss@example.com","password":"pass123"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/owner/login", bytes.NewBufferString(body))

	h.Login(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "owner.jwt.token", resp["token"])
	ownerSvc.AssertExpectations(t)
	issuer.AssertExpectations(t)
}

func TestOwnerLogin_InvalidCredentials(t *testing.T) {
	ownerSvc := new(mockOwnerService)
	issuer := new(mockTokenIssuer)
	h := NewOwnerAuthHandler(ownerSvc, issuer, "")

	ownerSvc.On("Authenticate", matchAny, "boss@example.com", "wrong").
		Return(nil, owner.ErrInvalidCredentials)

	body := `{"email":"boss@example.com","password":"wrong"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/owner/login", bytes.NewBufferString(body))

	h.Login(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	ownerSvc.AssertExpectations(t)
}

// --- OwnerBootstrap ---

func TestOwnerBootstrap_Success(t *testing.T) {
	ownerSvc := new(mockOwnerService)
	issuer := new(mockTokenIssuer)
	h := NewOwnerAuthHandler(ownerSvc, issuer, "mysecret")

	ownerID := uuid.New()
	o := &owner.Owner{ID: ownerID, Name: "First Owner", Email: "first@example.com"}

	ownerSvc.On("Count", matchAny).Return(0, nil)
	ownerSvc.On("Create", matchAny, "First Owner", "first@example.com", "strongpass").Return(o, nil)
	issuer.On("Issue", auth.Claims{
		UserID:  ownerID,
		Role:    auth.RoleAdmin,
		OwnerID: ownerID,
	}, 8*time.Hour).Return("bootstrap.jwt.token", nil)

	body := `{"name":"First Owner","email":"first@example.com","password":"strongpass"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/owner/bootstrap", bytes.NewBufferString(body))
	r.Header.Set("X-Bootstrap-Secret", "mysecret")

	h.Bootstrap(w, r)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "bootstrap.jwt.token", resp["token"])
	ownerSvc.AssertExpectations(t)
	issuer.AssertExpectations(t)
}

func TestOwnerBootstrap_WrongSecret(t *testing.T) {
	ownerSvc := new(mockOwnerService)
	issuer := new(mockTokenIssuer)
	h := NewOwnerAuthHandler(ownerSvc, issuer, "mysecret")

	body := `{"name":"Owner","email":"o@example.com","password":"pass"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/owner/bootstrap", bytes.NewBufferString(body))
	r.Header.Set("X-Bootstrap-Secret", "wrongsecret")

	h.Bootstrap(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
	ownerSvc.AssertNotCalled(t, "Create")
}

func TestOwnerBootstrap_AlreadyExists(t *testing.T) {
	ownerSvc := new(mockOwnerService)
	issuer := new(mockTokenIssuer)
	h := NewOwnerAuthHandler(ownerSvc, issuer, "mysecret")

	ownerSvc.On("Count", matchAny).Return(1, nil)

	body := `{"name":"Owner","email":"o@example.com","password":"pass"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/owner/bootstrap", bytes.NewBufferString(body))
	r.Header.Set("X-Bootstrap-Secret", "mysecret")

	h.Bootstrap(w, r)

	assert.Equal(t, http.StatusConflict, w.Code)
	ownerSvc.AssertNotCalled(t, "Create")
	ownerSvc.AssertExpectations(t)
}
