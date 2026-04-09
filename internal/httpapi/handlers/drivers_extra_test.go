package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/bmunoz/gentax/internal/driver"
)

func TestDriverHandler_List_Success(t *testing.T) {
	driverSvc := &mockDriverService{}
	h := NewDriverHandler(driverSvc)

	claims, ownerID := adminClaims()
	drivers := []*driver.Driver{
		{ID: uuid.New(), OwnerID: ownerID, FullName: "Alice"},
	}
	driverSvc.On("List", matchAny, ownerID).Return(drivers, nil)

	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodGet, "/drivers", claims)
	h.List(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
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
