package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bmunoz/gentax/internal/auth"
	"github.com/bmunoz/gentax/internal/driver"
	mw "github.com/bmunoz/gentax/internal/httpapi/middleware"
	"github.com/bmunoz/gentax/internal/taxi"
)

// TaxiHandler handles /taxis endpoints.
type TaxiHandler struct {
	taxiSvc   taxi.Service
	driverSvc driver.Service
}

// NewTaxiHandler constructs a TaxiHandler.
func NewTaxiHandler(taxiSvc taxi.Service, driverSvc driver.Service) *TaxiHandler {
	return &TaxiHandler{taxiSvc: taxiSvc, driverSvc: driverSvc}
}

// createTaxiRequest is the request body for POST /taxis.
type createTaxiRequest struct {
	Plate string `json:"plate"`
	Model string `json:"model"`
	Year  int    `json:"year"`
}

// List handles GET /taxis — returns all taxis for the authenticated owner.
func (h *TaxiHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	taxis, err := h.taxiSvc.List(r.Context(), claims.OwnerID)
	if err != nil {
		mw.DomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, taxis)
}

// Create handles POST /taxis — creates a new taxi for the authenticated owner.
func (h *TaxiHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	var req createTaxiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	// owner_id MUST come from JWT claims — never from the request body (REQ-FRD-04).
	input := taxi.CreateInput{
		OwnerID: claims.OwnerID,
		Plate:   req.Plate,
		Model:   req.Model,
		Year:    req.Year,
	}

	t, err := h.taxiSvc.Create(r.Context(), input)
	if err != nil {
		mw.DomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, t)
}

// Deactivate handles DELETE /taxis/{id} — soft-deletes a taxi.
func (h *TaxiHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid taxi id", "bad_request")
		return
	}

	if err := h.taxiSvc.Deactivate(r.Context(), id, claims.OwnerID); err != nil {
		mw.DomainError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AssignDriver handles POST /taxis/{id}/assign/{driverID}.
func (h *TaxiHandler) AssignDriver(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	taxiID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid taxi id", "bad_request")
		return
	}

	driverID, err := uuid.Parse(chi.URLParam(r, "driverID"))
	if err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid driver id", "bad_request")
		return
	}

	// Delegate to driver.Service.AssignTaxi (ownership verified inside).
	if err := h.driverSvc.AssignTaxi(r.Context(), driverID, taxiID, claims.OwnerID); err != nil {
		mw.DomainError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UnassignDriver handles DELETE /taxis/{id}/assign/{driverID}.
func (h *TaxiHandler) UnassignDriver(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	driverID, err := uuid.Parse(chi.URLParam(r, "driverID"))
	if err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid driver id", "bad_request")
		return
	}

	if err := h.driverSvc.UnassignTaxi(r.Context(), driverID, claims.OwnerID); err != nil {
		mw.DomainError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
