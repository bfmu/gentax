package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bmunoz/gentax/internal/expense"
)

func newReportRouter(h *ReportHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/reports/expenses", h.ExpenseList)
	r.Get("/reports/taxis", h.TaxiSummary)
	r.Get("/reports/drivers", h.DriverSummary)
	r.Get("/reports/categories", h.CategorySummary)
	return r
}

// TestReportHandler_TaxiSummary_MissingDateRange asserts that missing date_from/date_to returns 422.
func TestReportHandler_TaxiSummary_MissingDateRange(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewReportHandler(expSvc)

	claims, _ := adminClaims()

	rtr := newReportRouter(h)

	tests := []struct {
		name string
		url  string
	}{
		{"both missing", "/reports/taxis"},
		{"date_to missing", "/reports/taxis?date_from=2024-01-01"},
		{"date_from missing", "/reports/taxis?date_to=2024-01-31"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := newAuthRequest(http.MethodGet, tt.url, claims)
			rtr.ServeHTTP(w, r)
			assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		})
	}

	expSvc.AssertNotCalled(t, "SumByTaxi")
}

// TestReportHandler_TaxiSummary_Success asserts that a valid request returns 200 with a summary array.
func TestReportHandler_TaxiSummary_Success(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewReportHandler(expSvc)

	claims, ownerID := adminClaims()

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	summaries := []*expense.TaxiSummary{
		{TaxiID: uuid.New(), TaxiPlate: "ABC123", Total: decimal.NewFromInt(500000), Count: 3},
		{TaxiID: uuid.New(), TaxiPlate: "XYZ999", Total: decimal.NewFromInt(0), Count: 0},
	}
	expSvc.On("SumByTaxi", matchAny, ownerID, from, to).Return(summaries, nil)

	rtr := newReportRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodGet, "/reports/taxis?date_from=2024-01-01&date_to=2024-01-31", claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var got []expense.TaxiSummary
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Len(t, got, 2)
	assert.Equal(t, "ABC123", got[0].TaxiPlate)

	expSvc.AssertExpectations(t)
}

// TestReportHandler_DriverSummary_MissingDateRange verifies 422 for missing dates.
func TestReportHandler_DriverSummary_MissingDateRange(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewReportHandler(expSvc)
	claims, _ := adminClaims()

	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodGet, "/reports/drivers", claims)
	h.DriverSummary(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// TestReportHandler_CategorySummary_Success verifies that a category summary is returned.
func TestReportHandler_CategorySummary_Success(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewReportHandler(expSvc)

	claims, ownerID := adminClaims()
	from := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC)

	cats := []*expense.CategorySummary{
		{CategoryID: uuid.New(), CategoryName: "Fuel", Total: decimal.NewFromInt(200000), Count: 5},
	}
	expSvc.On("SumByCategory", matchAny, ownerID, from, to).Return(cats, nil)

	rtr := newReportRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodGet, "/reports/categories?date_from=2024-03-01&date_to=2024-03-31", claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	expSvc.AssertExpectations(t)
}
