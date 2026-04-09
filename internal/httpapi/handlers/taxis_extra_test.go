package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/bmunoz/gentax/internal/driver"
	"github.com/bmunoz/gentax/internal/taxi"
)

func TestTaxiHandler_List_Success(t *testing.T) {
	taxiSvc := &mockTaxiService{}
	driverSvc := &mockDriverService{}
	h := NewTaxiHandler(taxiSvc, driverSvc)

	claims, ownerID := adminClaims()
	taxis := []*taxi.Taxi{
		{ID: uuid.New(), OwnerID: ownerID, Plate: "AAA001"},
	}
	taxiSvc.On("List", matchAny, ownerID).Return(taxis, nil)

	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodGet, "/taxis", claims)
	h.List(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	taxiSvc.AssertExpectations(t)
}

func TestTaxiHandler_Deactivate_Success(t *testing.T) {
	taxiSvc := &mockTaxiService{}
	driverSvc := &mockDriverService{}
	h := NewTaxiHandler(taxiSvc, driverSvc)

	claims, ownerID := adminClaims()
	taxiID := uuid.New()

	taxiSvc.On("Deactivate", matchAny, taxiID, ownerID).Return(nil)

	rtr := newTaxiRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodDelete, "/taxis/"+taxiID.String(), claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	taxiSvc.AssertExpectations(t)
}

func TestTaxiHandler_AssignDriver_Success(t *testing.T) {
	taxiSvc := &mockTaxiService{}
	driverSvc := &mockDriverService{}
	h := NewTaxiHandler(taxiSvc, driverSvc)

	claims, ownerID := adminClaims()
	taxiID := uuid.New()
	driverID := uuid.New()

	driverSvc.On("AssignTaxi", matchAny, driverID, taxiID, ownerID).Return(nil)

	rtr := newTaxiRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodPost, "/taxis/"+taxiID.String()+"/assign/"+driverID.String(), claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	driverSvc.AssertExpectations(t)
}

func TestTaxiHandler_UnassignDriver_Success(t *testing.T) {
	taxiSvc := &mockTaxiService{}
	driverSvc := &mockDriverService{}
	h := NewTaxiHandler(taxiSvc, driverSvc)

	claims, ownerID := adminClaims()
	taxiID := uuid.New()
	driverID := uuid.New()

	driverSvc.On("UnassignTaxi", matchAny, driverID, ownerID).Return(nil)

	rtr := newTaxiRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodDelete, "/taxis/"+taxiID.String()+"/assign/"+driverID.String(), claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	driverSvc.AssertExpectations(t)
}

func TestTaxiHandler_AssignDriver_AlreadyAssigned(t *testing.T) {
	taxiSvc := &mockTaxiService{}
	driverSvc := &mockDriverService{}
	h := NewTaxiHandler(taxiSvc, driverSvc)

	claims, ownerID := adminClaims()
	taxiID := uuid.New()
	driverID := uuid.New()

	driverSvc.On("AssignTaxi", matchAny, driverID, taxiID, ownerID).Return(driver.ErrAlreadyAssigned)

	rtr := newTaxiRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodPost, "/taxis/"+taxiID.String()+"/assign/"+driverID.String(), claims)
	rtr.ServeHTTP(w, r)

	// ErrAlreadyAssigned falls through to 500 since it's not in DomainError mapping;
	// verify we still get an error response (500 internal_error).
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	driverSvc.AssertExpectations(t)
}

func TestTaxiHandler_UnassignDriver_NotFound(t *testing.T) {
	taxiSvc := &mockTaxiService{}
	driverSvc := &mockDriverService{}
	h := NewTaxiHandler(taxiSvc, driverSvc)

	claims, ownerID := adminClaims()
	taxiID := uuid.New()
	driverID := uuid.New()

	driverSvc.On("UnassignTaxi", matchAny, driverID, ownerID).Return(driver.ErrNotFound)

	rtr := newTaxiRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodDelete, "/taxis/"+taxiID.String()+"/assign/"+driverID.String(), claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assertErrorCode(t, w, "not_found")
	driverSvc.AssertExpectations(t)
}
