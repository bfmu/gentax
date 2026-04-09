package handlers

import (
	"net/http"
	"time"

	"github.com/bmunoz/gentax/internal/auth"
	"github.com/bmunoz/gentax/internal/expense"
	mw "github.com/bmunoz/gentax/internal/httpapi/middleware"
)

// ReportHandler handles /reports endpoints.
type ReportHandler struct {
	expenseSvc expense.Service
}

// NewReportHandler constructs a ReportHandler.
func NewReportHandler(expenseSvc expense.Service) *ReportHandler {
	return &ReportHandler{expenseSvc: expenseSvc}
}

// ExpenseList handles GET /reports/expenses — same as GET /expenses but for reporting.
func (h *ReportHandler) ExpenseList(w http.ResponseWriter, r *http.Request) {
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

// TaxiSummary handles GET /reports/taxis — sum of approved expenses per taxi.
// date_from and date_to are required query parameters.
func (h *ReportHandler) TaxiSummary(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	from, to, ok := parseDateRange(w, r)
	if !ok {
		return
	}

	summaries, err := h.expenseSvc.SumByTaxi(r.Context(), claims.OwnerID, from, to)
	if err != nil {
		mw.DomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, summaries)
}

// DriverSummary handles GET /reports/drivers — sum of approved expenses per driver.
func (h *ReportHandler) DriverSummary(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	from, to, ok := parseDateRange(w, r)
	if !ok {
		return
	}

	summaries, err := h.expenseSvc.SumByDriver(r.Context(), claims.OwnerID, from, to)
	if err != nil {
		mw.DomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, summaries)
}

// CategorySummary handles GET /reports/categories — sum of approved expenses per category.
func (h *ReportHandler) CategorySummary(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	from, to, ok := parseDateRange(w, r)
	if !ok {
		return
	}

	summaries, err := h.expenseSvc.SumByCategory(r.Context(), claims.OwnerID, from, to)
	if err != nil {
		mw.DomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, summaries)
}

// parseDateRange extracts and validates date_from/date_to query params.
// Both are required. Returns false and writes a 422 if either is missing or invalid.
func parseDateRange(w http.ResponseWriter, r *http.Request) (from, to time.Time, ok bool) {
	q := r.URL.Query()

	fromStr := q.Get("date_from")
	toStr := q.Get("date_to")

	if fromStr == "" || toStr == "" {
		mw.WriteError(w, http.StatusUnprocessableEntity, "date_from and date_to are required", "invalid_input")
		return time.Time{}, time.Time{}, false
	}

	from, err := time.Parse(time.DateOnly, fromStr)
	if err != nil {
		mw.WriteError(w, http.StatusUnprocessableEntity, "invalid date_from format (expected YYYY-MM-DD)", "invalid_input")
		return time.Time{}, time.Time{}, false
	}

	to, err = time.Parse(time.DateOnly, toStr)
	if err != nil {
		mw.WriteError(w, http.StatusUnprocessableEntity, "invalid date_to format (expected YYYY-MM-DD)", "invalid_input")
		return time.Time{}, time.Time{}, false
	}

	return from, to, true
}
