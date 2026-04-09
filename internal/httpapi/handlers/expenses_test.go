package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bmunoz/gentax/internal/expense"
)

func newExpenseRouter(h *ExpenseHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/expenses", h.List)
	r.Get("/expenses/{id}", h.GetByID)
	r.Post("/expenses", h.Create)
	r.Patch("/expenses/{id}/approve", h.Approve)
	r.Patch("/expenses/{id}/reject", h.Reject)
	return r
}

// TestExpenseHandler_Create_MissingReceiptID verifies that creating an expense without
// a receipt_id returns 422 (REQ-FRD-01).
func TestExpenseHandler_Create_MissingReceiptID(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewExpenseHandler(expSvc)

	claims, _, _ := driverClaims()

	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodPost, "/expenses", claims)
	// receipt_id is zero (uuid.Nil) — should be rejected before service call.
	r.Body = jsonBody(map[string]string{
		"taxi_id":     uuid.New().String(),
		"category_id": uuid.New().String(),
		// receipt_id intentionally omitted → will be uuid.Nil after decode
	})

	h.Create(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assertErrorCode(t, w, "receipt_required")
	// Service must NOT be called when receipt_id is missing.
	expSvc.AssertNotCalled(t, "Create")
}

// TestExpenseHandler_Create_OwnerAndDriverFromClaims asserts that owner_id and driver_id
// are sourced from JWT claims, not from the request body (REQ-FRD-04).
func TestExpenseHandler_Create_OwnerAndDriverFromClaims(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewExpenseHandler(expSvc)

	claims, ownerID, driverID := driverClaims()

	taxiID := uuid.New()
	catID := uuid.New()
	receiptID := uuid.New()

	expected := &expense.Expense{
		ID:         uuid.New(),
		OwnerID:    ownerID,
		DriverID:   driverID,
		TaxiID:     taxiID,
		CategoryID: catID,
		ReceiptID:  receiptID,
		Status:     expense.StatusPending,
	}

	// Service must receive ownerID and driverID from claims, not from body.
	expSvc.On("Create", matchAny, expense.CreateInput{
		OwnerID:    ownerID,
		DriverID:   driverID,
		TaxiID:     taxiID,
		CategoryID: catID,
		ReceiptID:  receiptID,
		Notes:      "",
	}).Return(expected, nil)

	body := map[string]string{
		"taxi_id":     taxiID.String(),
		"category_id": catID.String(),
		"receipt_id":  receiptID.String(),
		// Attacker tries to override owner_id and driver_id — must be ignored.
		"owner_id":  uuid.New().String(),
		"driver_id": uuid.New().String(),
	}
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodPost, "/expenses", claims)
	r.Body = jsonBody(body)

	h.Create(w, r)

	assert.Equal(t, http.StatusCreated, w.Code)
	expSvc.AssertExpectations(t)
}

// TestExpenseHandler_Approve_WrongStatus asserts that approving a non-confirmed expense returns 409.
func TestExpenseHandler_Approve_WrongStatus(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewExpenseHandler(expSvc)

	claims, ownerID := adminClaims()
	expID := uuid.New()

	expSvc.On("Approve", matchAny, expID, ownerID).Return(expense.ErrInvalidTransition)

	rtr := newExpenseRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodPatch, "/expenses/"+expID.String()+"/approve", claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusConflict, w.Code)
	assertErrorCode(t, w, "invalid_transition")
	expSvc.AssertExpectations(t)
}

// TestExpenseHandler_Reject_Success asserts that a successful rejection returns 200.
func TestExpenseHandler_Reject_Success(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewExpenseHandler(expSvc)

	claims, ownerID := adminClaims()
	expID := uuid.New()

	expSvc.On("Reject", matchAny, expID, ownerID, "wrong fuel type").Return(nil)

	rtr := newExpenseRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodPatch, "/expenses/"+expID.String()+"/reject", claims)
	r.Body = jsonBody(map[string]string{"reason": "wrong fuel type"})
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	expSvc.AssertExpectations(t)
}

// TestExpenseHandler_List_ParsesFilters asserts that query parameters are correctly parsed
// into the ListFilter struct passed to the service.
func TestExpenseHandler_List_ParsesFilters(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewExpenseHandler(expSvc)

	claims, ownerID := adminClaims()
	driverID := uuid.New()

	expSvc.On("List", matchAny, matchAny).Return([]*expense.Expense{}, nil)

	rtr := newExpenseRouter(h)
	w := httptest.NewRecorder()
	url := "/expenses?driver_id=" + driverID.String() + "&status=approved&limit=10&offset=5"
	r := newAuthRequest(http.MethodGet, url, claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	// Capture the actual filter argument passed to the service.
	call := expSvc.Calls[0]
	require.Len(t, call.Arguments, 2)
	filter, ok := call.Arguments[1].(expense.ListFilter)
	require.True(t, ok)

	assert.Equal(t, ownerID, filter.OwnerID)
	require.NotNil(t, filter.DriverID)
	assert.Equal(t, driverID, *filter.DriverID)
	assert.Equal(t, 10, filter.Limit)
	assert.Equal(t, 5, filter.Offset)
	s := expense.StatusApproved
	require.NotNil(t, filter.Status)
	assert.Equal(t, s, *filter.Status)
}

// TestExpenseHandler_List_DefaultLimit asserts the default pagination limit is 20.
func TestExpenseHandler_List_DefaultLimit(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewExpenseHandler(expSvc)

	claims, _ := adminClaims()
	expSvc.On("List", matchAny, matchAny).Return([]*expense.Expense{}, nil)

	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodGet, "/expenses", claims)
	h.List(w, r)

	call := expSvc.Calls[0]
	filter := call.Arguments[1].(expense.ListFilter)
	assert.Equal(t, defaultLimit, filter.Limit)
}

// TestExpenseHandler_List_MaxLimit verifies the max cap of 100 is enforced.
func TestExpenseHandler_List_MaxLimit(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewExpenseHandler(expSvc)

	claims, _ := adminClaims()
	expSvc.On("List", matchAny, matchAny).Return([]*expense.Expense{}, nil)

	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodGet, "/expenses?limit=9999", claims)
	h.List(w, r)

	call := expSvc.Calls[0]
	filter := call.Arguments[1].(expense.ListFilter)
	assert.Equal(t, maxLimit, filter.Limit)
}

// TestExpenseHandler_GetByID_Success verifies a single expense is returned with 200.
func TestExpenseHandler_GetByID_Success(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewExpenseHandler(expSvc)

	claims, ownerID := adminClaims()
	expID := uuid.New()
	exp := &expense.Expense{ID: expID, OwnerID: ownerID}

	expSvc.On("GetByID", matchAny, expID, ownerID).Return(exp, nil)

	rtr := newExpenseRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodGet, "/expenses/"+expID.String(), claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var got expense.Expense
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, expID, got.ID)
	expSvc.AssertExpectations(t)
}
