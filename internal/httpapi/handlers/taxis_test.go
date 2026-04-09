package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bmunoz/gentax/internal/taxi"
)

// newTaxiRouter builds a minimal chi router for taxi handler tests.
func newTaxiRouter(h *TaxiHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/taxis", h.List)
	r.Post("/taxis", h.Create)
	r.Delete("/taxis/{id}", h.Deactivate)
	r.Post("/taxis/{id}/assign/{driverID}", h.AssignDriver)
	r.Delete("/taxis/{id}/assign/{driverID}", h.UnassignDriver)
	return r
}

func TestTaxiHandler_List_RequiresAuth(t *testing.T) {
	taxiSvc := &mockTaxiService{}
	driverSvc := &mockDriverService{}
	h := NewTaxiHandler(taxiSvc, driverSvc)

	// Request with no claims in context → handler should respond 401.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/taxis", nil)
	// Do not inject claims — simulates unauthenticated request.
	h.List(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestTaxiHandler_Create_Success(t *testing.T) {
	taxiSvc := &mockTaxiService{}
	driverSvc := &mockDriverService{}
	h := NewTaxiHandler(taxiSvc, driverSvc)

	claims, ownerID := adminClaims()

	expected := &taxi.Taxi{
		ID:      uuid.New(),
		OwnerID: ownerID,
		Plate:   "ABC123",
		Model:   "Toyota",
		Year:    2022,
		Active:  true,
		CreatedAt: time.Now(),
	}

	input := taxi.CreateInput{
		OwnerID: ownerID,
		Plate:   "ABC123",
		Model:   "Toyota",
		Year:    2022,
	}
	taxiSvc.On("Create", matchAny, input).Return(expected, nil)

	body := `{"plate":"ABC123","model":"Toyota","year":2022}`
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodPost, "/taxis", claims)
	r.Body = nopCloser(bytes.NewBufferString(body))

	h.Create(w, r)

	assert.Equal(t, http.StatusCreated, w.Code)
	var got taxi.Taxi
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, expected.Plate, got.Plate)
	taxiSvc.AssertExpectations(t)
}

func TestTaxiHandler_Create_DuplicatePlate(t *testing.T) {
	taxiSvc := &mockTaxiService{}
	driverSvc := &mockDriverService{}
	h := NewTaxiHandler(taxiSvc, driverSvc)

	claims, ownerID := adminClaims()
	input := taxi.CreateInput{OwnerID: ownerID, Plate: "ABC123", Model: "Toyota", Year: 2022}
	taxiSvc.On("Create", matchAny, input).Return(nil, taxi.ErrDuplicatePlate)

	body := `{"plate":"ABC123","model":"Toyota","year":2022}`
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodPost, "/taxis", claims)
	r.Body = nopCloser(bytes.NewBufferString(body))

	h.Create(w, r)

	assert.Equal(t, http.StatusConflict, w.Code)
	assertErrorCode(t, w, "duplicate_plate")
	taxiSvc.AssertExpectations(t)
}

func TestTaxiHandler_Create_InvalidYear(t *testing.T) {
	taxiSvc := &mockTaxiService{}
	driverSvc := &mockDriverService{}
	h := NewTaxiHandler(taxiSvc, driverSvc)

	claims, ownerID := adminClaims()
	input := taxi.CreateInput{OwnerID: ownerID, Plate: "ABC123", Model: "Toyota", Year: 1980}
	taxiSvc.On("Create", matchAny, input).Return(nil, taxi.ErrInvalidYear)

	body := `{"plate":"ABC123","model":"Toyota","year":1980}`
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodPost, "/taxis", claims)
	r.Body = nopCloser(bytes.NewBufferString(body))

	h.Create(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assertErrorCode(t, w, "invalid_year")
	taxiSvc.AssertExpectations(t)
}

// TestTaxiHandler_Create_OwnerIDFromClaims asserts that the ownerID passed to the service
// matches the JWT claims, not any value supplied in the request body (REQ-FRD-04).
func TestTaxiHandler_Create_OwnerIDFromClaims(t *testing.T) {
	taxiSvc := &mockTaxiService{}
	driverSvc := &mockDriverService{}
	h := NewTaxiHandler(taxiSvc, driverSvc)

	claims, ownerID := adminClaims()
	foreignOwner := uuid.New() // what an attacker might try to inject

	expected := &taxi.Taxi{ID: uuid.New(), OwnerID: ownerID, Plate: "XYZ999", Model: "Ford", Year: 2020, Active: true}

	// Service must receive ownerID from claims, not foreignOwner.
	taxiSvc.On("Create", matchAny, taxi.CreateInput{
		OwnerID: ownerID,
		Plate:   "XYZ999",
		Model:   "Ford",
		Year:    2020,
	}).Return(expected, nil)

	// Body includes a fake owner_id that should be ignored.
	body := `{"plate":"XYZ999","model":"Ford","year":2020,"owner_id":"` + foreignOwner.String() + `"}`
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodPost, "/taxis", claims)
	r.Body = nopCloser(bytes.NewBufferString(body))

	h.Create(w, r)

	assert.Equal(t, http.StatusCreated, w.Code)
	taxiSvc.AssertExpectations(t)
}

func TestTaxiHandler_Deactivate_NotFound(t *testing.T) {
	taxiSvc := &mockTaxiService{}
	driverSvc := &mockDriverService{}
	h := NewTaxiHandler(taxiSvc, driverSvc)

	claims, ownerID := adminClaims()
	taxiID := uuid.New()

	taxiSvc.On("Deactivate", matchAny, taxiID, ownerID).Return(taxi.ErrNotFound)

	rtr := newTaxiRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodDelete, "/taxis/"+taxiID.String(), claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assertErrorCode(t, w, "not_found")
	taxiSvc.AssertExpectations(t)
}
