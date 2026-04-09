package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bmunoz/gentax/internal/auth"
	"github.com/bmunoz/gentax/internal/expense"
	mw "github.com/bmunoz/gentax/internal/httpapi/middleware"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// ExpenseHandler handles /expenses endpoints.
type ExpenseHandler struct {
	expenseSvc expense.Service
}

// NewExpenseHandler constructs an ExpenseHandler.
func NewExpenseHandler(expenseSvc expense.Service) *ExpenseHandler {
	return &ExpenseHandler{expenseSvc: expenseSvc}
}

// createExpenseRequest is the request body for POST /expenses.
// owner_id and driver_id MUST NOT be in the request body — they come from JWT claims.
type createExpenseRequest struct {
	TaxiID     uuid.UUID `json:"taxi_id"`
	CategoryID uuid.UUID `json:"category_id"`
	ReceiptID  uuid.UUID `json:"receipt_id"`
	Notes      string    `json:"notes"`
}

// rejectExpenseRequest is the request body for PATCH /expenses/{id}/reject.
type rejectExpenseRequest struct {
	Reason string `json:"reason"`
}

// List handles GET /expenses — lists expenses with optional query-param filters.
// All expenses are scoped to the authenticated owner (REQ-TNT-01).
func (h *ExpenseHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	filter, err := parseListFilter(r, claims.OwnerID)
	if err != nil {
		mw.WriteError(w, http.StatusBadRequest, err.Error(), "bad_request")
		return
	}

	expenses, err := h.expenseSvc.List(r.Context(), filter)
	if err != nil {
		mw.DomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, expenses)
}

// GetByID handles GET /expenses/{id}.
func (h *ExpenseHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid expense id", "bad_request")
		return
	}

	exp, err := h.expenseSvc.GetByID(r.Context(), id, claims.OwnerID)
	if err != nil {
		mw.DomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, exp)
}

// Create handles POST /expenses (role=driver).
// owner_id and driver_id come exclusively from JWT claims (REQ-FRD-04).
func (h *ExpenseHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	// Driver must have a driver_id in claims.
	if claims.DriverID == nil {
		mw.WriteError(w, http.StatusForbidden, "forbidden: not a driver", "forbidden")
		return
	}

	var req createExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	// receipt_id must be provided (REQ-FRD-01).
	if req.ReceiptID == uuid.Nil {
		mw.DomainError(w, expense.ErrReceiptRequired)
		return
	}

	input := expense.CreateInput{
		OwnerID:    claims.OwnerID,    // from JWT — never from body
		DriverID:   *claims.DriverID, // from JWT — never from body
		TaxiID:     req.TaxiID,
		CategoryID: req.CategoryID,
		ReceiptID:  req.ReceiptID,
		Notes:      req.Notes,
	}

	exp, err := h.expenseSvc.Create(r.Context(), input)
	if err != nil {
		mw.DomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, exp)
}

// Approve handles PATCH /expenses/{id}/approve (role=admin).
func (h *ExpenseHandler) Approve(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid expense id", "bad_request")
		return
	}

	if err := h.expenseSvc.Approve(r.Context(), id, claims.OwnerID); err != nil {
		mw.DomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

// Reject handles PATCH /expenses/{id}/reject (role=admin).
func (h *ExpenseHandler) Reject(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid expense id", "bad_request")
		return
	}

	var req rejectExpenseRequest
	// Decode is optional — reason may be omitted.
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.expenseSvc.Reject(r.Context(), id, claims.OwnerID, req.Reason); err != nil {
		mw.DomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

// parseListFilter reads query parameters from the request and builds a ListFilter.
func parseListFilter(r *http.Request, ownerID uuid.UUID) (expense.ListFilter, error) {
	q := r.URL.Query()

	filter := expense.ListFilter{
		OwnerID: ownerID,
		Limit:   defaultLimit,
	}

	if v := q.Get("driver_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return filter, err
		}
		filter.DriverID = &id
	}

	if v := q.Get("taxi_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return filter, err
		}
		filter.TaxiID = &id
	}

	if v := q.Get("category_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return filter, err
		}
		filter.CategoryID = &id
	}

	if v := q.Get("status"); v != "" {
		s := expense.Status(v)
		filter.Status = &s
	}

	if v := q.Get("date_from"); v != "" {
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			return filter, err
		}
		filter.DateFrom = &t
	}

	if v := q.Get("date_to"); v != "" {
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			return filter, err
		}
		filter.DateTo = &t
	}

	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return filter, err
		}
		if n > maxLimit {
			n = maxLimit
		}
		if n > 0 {
			filter.Limit = n
		}
	}

	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return filter, err
		}
		if n >= 0 {
			filter.Offset = n
		}
	}

	return filter, nil
}
