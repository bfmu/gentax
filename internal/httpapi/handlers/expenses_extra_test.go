package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/bmunoz/gentax/internal/expense"
)

// Test the unauthenticated paths for expense handlers.

func TestExpenseHandler_List_RequiresAuth(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewExpenseHandler(expSvc)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodGet, "/expenses", nil)
	h.List(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestExpenseHandler_GetByID_RequiresAuth(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewExpenseHandler(expSvc)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodGet, "/expenses/"+uuid.New().String(), nil)
	h.GetByID(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestExpenseHandler_GetByID_NotFound(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewExpenseHandler(expSvc)

	claims, ownerID := adminClaims()
	expID := uuid.New()

	expSvc.On("GetByID", matchAny, expID, ownerID).Return(nil, expense.ErrNotFound)

	rtr := newExpenseRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodGet, "/expenses/"+expID.String(), claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assertErrorCode(t, w, "not_found")
	expSvc.AssertExpectations(t)
}

func TestExpenseHandler_Approve_RequiresAuth(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewExpenseHandler(expSvc)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodPatch, "/expenses/"+uuid.New().String()+"/approve", nil)
	h.Approve(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestExpenseHandler_Reject_RequiresAuth(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewExpenseHandler(expSvc)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodPatch, "/expenses/"+uuid.New().String()+"/reject", nil)
	h.Reject(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestExpenseHandler_Create_RequiresAuth(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewExpenseHandler(expSvc)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodPost, "/expenses", nil)
	h.Create(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestExpenseHandler_Create_AdminForbidden(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewExpenseHandler(expSvc)

	// Admin claims have no DriverID → 403 when trying to create expense.
	claims, _ := adminClaims()

	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodPost, "/expenses", claims)
	r.Body = jsonBody(map[string]string{
		"taxi_id":     uuid.New().String(),
		"category_id": uuid.New().String(),
		"receipt_id":  uuid.New().String(),
	})
	h.Create(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestExpenseHandler_List_InvalidDriverID(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewExpenseHandler(expSvc)

	claims, _ := adminClaims()

	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodGet, "/expenses?driver_id=not-a-uuid", claims)
	h.List(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExpenseHandler_Approve_Success(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewExpenseHandler(expSvc)

	claims, ownerID := adminClaims()
	expID := uuid.New()
	expSvc.On("Approve", matchAny, expID, ownerID).Return(nil)

	rtr := newExpenseRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodPatch, "/expenses/"+expID.String()+"/approve", claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	expSvc.AssertExpectations(t)
}
