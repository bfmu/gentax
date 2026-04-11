package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bmunoz/gentax/internal/driver"
)

func TestDriverHandler_List_Success(t *testing.T) {
	driverSvc := &mockDriverService{}
	h := NewDriverHandler(driverSvc)

	claims, ownerID := adminClaims()
	driverID := uuid.New()
	taxiID := uuid.New()

	results := []*driver.DriverWithAssignment{
		{
			Driver:       &driver.Driver{ID: driverID, OwnerID: ownerID, FullName: "Alice"},
			AssignedTaxi: &driver.AssignedTaxiView{ID: taxiID, Plate: "XYZ-999"},
		},
	}
	driverSvc.On("ListWithAssignment", matchAny, ownerID).Return(results, nil)

	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodGet, "/drivers", claims)
	h.List(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	driverSvc.AssertExpectations(t)
}

// TestDriverHandler_List_IncludesAssignment verifies that the response body contains
// the assigned_taxi field with the correct plate when a driver has an active assignment.
func TestDriverHandler_List_IncludesAssignment(t *testing.T) {
	driverSvc := &mockDriverService{}
	h := NewDriverHandler(driverSvc)

	claims, ownerID := adminClaims()
	driverID := uuid.New()
	taxiID := uuid.New()

	results := []*driver.DriverWithAssignment{
		{
			Driver:       &driver.Driver{ID: driverID, OwnerID: ownerID, FullName: "Carlos"},
			AssignedTaxi: &driver.AssignedTaxiView{ID: taxiID, Plate: "ABC-123"},
		},
	}
	driverSvc.On("ListWithAssignment", matchAny, ownerID).Return(results, nil)

	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodGet, "/drivers", claims)
	h.List(w, r)

	require.Equal(t, http.StatusOK, w.Code)

	var body []map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	require.Len(t, body, 1)

	assignedTaxi, ok := body[0]["assigned_taxi"].(map[string]interface{})
	require.True(t, ok, "expected assigned_taxi to be an object")
	assert.Equal(t, "ABC-123", assignedTaxi["plate"])
	assert.Equal(t, taxiID.String(), assignedTaxi["id"])

	driverSvc.AssertExpectations(t)
}

// TestDriverHandler_List_AssignedTaxiNil verifies that a driver without an assignment
// has assigned_taxi: null in the JSON response.
func TestDriverHandler_List_AssignedTaxiNil(t *testing.T) {
	driverSvc := &mockDriverService{}
	h := NewDriverHandler(driverSvc)

	claims, ownerID := adminClaims()
	driverID := uuid.New()

	results := []*driver.DriverWithAssignment{
		{
			Driver:       &driver.Driver{ID: driverID, OwnerID: ownerID, FullName: "Sin taxi"},
			AssignedTaxi: nil,
		},
	}
	driverSvc.On("ListWithAssignment", matchAny, ownerID).Return(results, nil)

	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodGet, "/drivers", claims)
	h.List(w, r)

	require.Equal(t, http.StatusOK, w.Code)

	var body []map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	require.Len(t, body, 1)

	// assigned_taxi must be present and null.
	assignedTaxi, exists := body[0]["assigned_taxi"]
	assert.True(t, exists, "assigned_taxi key must be present in response")
	assert.Nil(t, assignedTaxi, "assigned_taxi must be null for unassigned driver")

	driverSvc.AssertExpectations(t)
}

func TestDriverHandler_List_RequiresAuth(t *testing.T) {
	driverSvc := &mockDriverService{}
	h := NewDriverHandler(driverSvc)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodGet, "/drivers", nil)
	h.List(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDriverHandler_Deactivate_Success(t *testing.T) {
	driverSvc := &mockDriverService{}
	h := NewDriverHandler(driverSvc)

	claims, ownerID := adminClaims()
	driverID := uuid.New()

	driverSvc.On("Deactivate", matchAny, driverID, ownerID).Return(nil)

	rtr := newDriverRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodDelete, "/drivers/"+driverID.String(), claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	driverSvc.AssertExpectations(t)
}

func TestDriverHandler_GenerateLinkToken_NotFound(t *testing.T) {
	driverSvc := &mockDriverService{}
	h := NewDriverHandler(driverSvc)

	claims, ownerID := adminClaims()
	driverID := uuid.New()

	driverSvc.On("GenerateLinkToken", matchAny, driverID, ownerID).Return("", driver.ErrNotFound)

	rtr := newDriverRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodPost, "/drivers/"+driverID.String()+"/link-token", claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assertErrorCode(t, w, "not_found")
	driverSvc.AssertExpectations(t)
}
