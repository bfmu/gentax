package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/bmunoz/gentax/internal/driver"
)

func newDriverRouter(h *DriverHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/drivers", h.List)
	r.Post("/drivers", h.Create)
	r.Delete("/drivers/{id}", h.Deactivate)
	r.Post("/drivers/{id}/link-token", h.GenerateLinkToken)
	return r
}

func TestDriverHandler_Create_BlankName(t *testing.T) {
	driverSvc := &mockDriverService{}
	h := NewDriverHandler(driverSvc)

	claims, ownerID := adminClaims()
	input := driver.CreateInput{OwnerID: ownerID, FullName: "", Phone: "123"}
	driverSvc.On("Create", matchAny, input).Return(nil, driver.ErrInvalidInput)

	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodPost, "/drivers", claims)
	r.Body = jsonBody(map[string]string{"full_name": "", "phone": "123"})

	h.Create(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assertErrorCode(t, w, "invalid_input")
	driverSvc.AssertExpectations(t)
}

func TestDriverHandler_Create_Success(t *testing.T) {
	driverSvc := &mockDriverService{}
	h := NewDriverHandler(driverSvc)

	claims, ownerID := adminClaims()

	expected := &driver.Driver{
		ID:       uuid.New(),
		OwnerID:  ownerID,
		FullName: "John Doe",
		Phone:    "3001234567",
		Active:   true,
	}

	input := driver.CreateInput{OwnerID: ownerID, FullName: "John Doe", Phone: "3001234567"}
	driverSvc.On("Create", matchAny, input).Return(expected, nil)

	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodPost, "/drivers", claims)
	r.Body = jsonBody(map[string]string{"full_name": "John Doe", "phone": "3001234567"})

	h.Create(w, r)

	assert.Equal(t, http.StatusCreated, w.Code)
	driverSvc.AssertExpectations(t)
}

func TestDriverHandler_GenerateLinkToken_Success(t *testing.T) {
	driverSvc := &mockDriverService{}
	h := NewDriverHandler(driverSvc)

	claims, ownerID := adminClaims()
	driverID := uuid.New()

	driverSvc.On("GenerateLinkToken", matchAny, driverID, ownerID).Return("my-link-token", nil)

	rtr := newDriverRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodPost, "/drivers/"+driverID.String()+"/link-token", claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	driverSvc.AssertExpectations(t)
}

func TestDriverHandler_Deactivate_NotFound(t *testing.T) {
	driverSvc := &mockDriverService{}
	h := NewDriverHandler(driverSvc)

	claims, ownerID := adminClaims()
	driverID := uuid.New()

	driverSvc.On("Deactivate", matchAny, driverID, ownerID).Return(driver.ErrNotFound)

	rtr := newDriverRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodDelete, "/drivers/"+driverID.String(), claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assertErrorCode(t, w, "not_found")
	driverSvc.AssertExpectations(t)
}
