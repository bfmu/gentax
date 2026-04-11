package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bmunoz/gentax/internal/auth"
	"github.com/bmunoz/gentax/internal/driver"
	mw "github.com/bmunoz/gentax/internal/httpapi/middleware"
)

// DriverHandler handles /drivers endpoints.
type DriverHandler struct {
	driverSvc driver.Service
}

// NewDriverHandler constructs a DriverHandler.
func NewDriverHandler(driverSvc driver.Service) *DriverHandler {
	return &DriverHandler{driverSvc: driverSvc}
}

// createDriverRequest is the request body for POST /drivers.
type createDriverRequest struct {
	FullName string `json:"full_name"`
	Phone    string `json:"phone"`
}

// linkTokenResponse is the response body for POST /drivers/{id}/link-token.
type linkTokenResponse struct {
	Token string `json:"token"`
}

// List handles GET /drivers — returns all drivers with their active taxi assignment.
func (h *DriverHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	drivers, err := h.driverSvc.ListWithAssignment(r.Context(), claims.OwnerID)
	if err != nil {
		mw.DomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, drivers)
}

// Create handles POST /drivers — creates a new driver for the authenticated owner.
// owner_id comes exclusively from JWT claims (REQ-FRD-04).
func (h *DriverHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	var req createDriverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	input := driver.CreateInput{
		OwnerID:  claims.OwnerID,
		FullName: req.FullName,
		Phone:    req.Phone,
	}

	drv, err := h.driverSvc.Create(r.Context(), input)
	if err != nil {
		mw.DomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, drv)
}

// Deactivate handles DELETE /drivers/{id} — soft-deletes a driver.
func (h *DriverHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid driver id", "bad_request")
		return
	}

	if err := h.driverSvc.Deactivate(r.Context(), id, claims.OwnerID); err != nil {
		mw.DomainError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GenerateLinkToken handles POST /drivers/{id}/link-token — generates a single-use link
// token for the driver so they can complete the Telegram /start bot flow.
func (h *DriverHandler) GenerateLinkToken(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid driver id", "bad_request")
		return
	}

	token, err := h.driverSvc.GenerateLinkToken(r.Context(), id, claims.OwnerID)
	if err != nil {
		mw.DomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, linkTokenResponse{Token: token})
}
