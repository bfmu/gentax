package middleware_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bmunoz/gentax/internal/driver"
	"github.com/bmunoz/gentax/internal/expense"
	"github.com/bmunoz/gentax/internal/httpapi/middleware"
	"github.com/bmunoz/gentax/internal/taxi"
)

// errorBody is the JSON shape returned by WriteError and DomainError.
type errorBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func decodeErrorBody(t *testing.T, rec *httptest.ResponseRecorder) errorBody {
	t.Helper()
	var body errorBody
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	return body
}

// ─── WriteError ───────────────────────────────────────────────────────────────

func TestWriteError_WritesJSONWithCorrectStatusAndBody(t *testing.T) {
	rec := httptest.NewRecorder()
	middleware.WriteError(rec, http.StatusNotFound, "not found", "not_found")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	body := decodeErrorBody(t, rec)
	assert.Equal(t, "not found", body.Error)
	assert.Equal(t, "not_found", body.Code)
}

// ─── DomainError — taxi ───────────────────────────────────────────────────────

func TestDomainError_TaxiNotFound_Returns404(t *testing.T) {
	rec := httptest.NewRecorder()
	middleware.DomainError(rec, taxi.ErrNotFound)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	body := decodeErrorBody(t, rec)
	assert.Equal(t, "not_found", body.Code)
}

func TestDomainError_TaxiDuplicatePlate_Returns409(t *testing.T) {
	rec := httptest.NewRecorder()
	middleware.DomainError(rec, taxi.ErrDuplicatePlate)
	assert.Equal(t, http.StatusConflict, rec.Code)
	body := decodeErrorBody(t, rec)
	assert.Equal(t, "duplicate_plate", body.Code)
}

func TestDomainError_TaxiInvalidYear_Returns422(t *testing.T) {
	rec := httptest.NewRecorder()
	middleware.DomainError(rec, taxi.ErrInvalidYear)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	body := decodeErrorBody(t, rec)
	assert.Equal(t, "invalid_year", body.Code)
}

// ─── DomainError — driver ─────────────────────────────────────────────────────

func TestDomainError_DriverNotFound_Returns404(t *testing.T) {
	rec := httptest.NewRecorder()
	middleware.DomainError(rec, driver.ErrNotFound)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	body := decodeErrorBody(t, rec)
	assert.Equal(t, "not_found", body.Code)
}

func TestDomainError_DriverInvalidInput_Returns422(t *testing.T) {
	rec := httptest.NewRecorder()
	middleware.DomainError(rec, driver.ErrInvalidInput)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	body := decodeErrorBody(t, rec)
	assert.Equal(t, "invalid_input", body.Code)
}

func TestDomainError_DriverDuplicateTelegram_Returns409(t *testing.T) {
	rec := httptest.NewRecorder()
	middleware.DomainError(rec, driver.ErrDuplicateTelegram)
	assert.Equal(t, http.StatusConflict, rec.Code)
	body := decodeErrorBody(t, rec)
	assert.Equal(t, "duplicate_telegram", body.Code)
}

// ─── DomainError — expense ────────────────────────────────────────────────────

func TestDomainError_ExpenseReceiptRequired_Returns422(t *testing.T) {
	rec := httptest.NewRecorder()
	middleware.DomainError(rec, expense.ErrReceiptRequired)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	body := decodeErrorBody(t, rec)
	assert.Equal(t, "receipt_required", body.Code)
}

func TestDomainError_ExpenseInvalidTransition_Returns409(t *testing.T) {
	rec := httptest.NewRecorder()
	middleware.DomainError(rec, expense.ErrInvalidTransition)
	assert.Equal(t, http.StatusConflict, rec.Code)
	body := decodeErrorBody(t, rec)
	assert.Equal(t, "invalid_transition", body.Code)
}

// ─── DomainError — fallthrough ────────────────────────────────────────────────

func TestDomainError_UnknownError_Returns500(t *testing.T) {
	rec := httptest.NewRecorder()
	middleware.DomainError(rec, errors.New("unexpected internal failure"))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	body := decodeErrorBody(t, rec)
	assert.Equal(t, "internal_error", body.Code)
	assert.Equal(t, "internal server error", body.Error)
}
