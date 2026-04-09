package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"github.com/bmunoz/gentax/internal/expense"
)

func TestReportHandler_ExpenseList_Success(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewReportHandler(expSvc)

	claims, _ := adminClaims()
	expSvc.On("List", matchAny, matchAny).Return([]*expense.Expense{}, nil)

	rtr := newReportRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodGet, "/reports/expenses", claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	expSvc.AssertExpectations(t)
}

func TestReportHandler_ExpenseList_RequiresAuth(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewReportHandler(expSvc)

	rtr := newReportRouter(h)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodGet, "/reports/expenses", nil)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestReportHandler_DriverSummary_Success(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewReportHandler(expSvc)

	claims, ownerID := adminClaims()
	from := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)

	summaries := []*expense.DriverSummary{
		{DriverID: uuid.New(), DriverName: "Alice", Total: decimal.NewFromInt(100000), Count: 2},
	}
	expSvc.On("SumByDriver", matchAny, ownerID, from, to).Return(summaries, nil)

	rtr := newReportRouter(h)
	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodGet, "/reports/drivers?date_from=2024-02-01&date_to=2024-02-29", claims)
	rtr.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	expSvc.AssertExpectations(t)
}

func TestReportHandler_TaxiSummary_RequiresAuth(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewReportHandler(expSvc)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodGet, "/reports/taxis?date_from=2024-01-01&date_to=2024-01-31", nil)
	h.TaxiSummary(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestReportHandler_CategorySummary_MissingDateRange(t *testing.T) {
	expSvc := &mockExpenseService{}
	h := NewReportHandler(expSvc)

	claims, _ := adminClaims()

	w := httptest.NewRecorder()
	r := newAuthRequest(http.MethodGet, "/reports/categories", claims)
	h.CategorySummary(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
