// Package middleware provides shared HTTP middleware and helpers for the gentax REST API.
package middleware

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bmunoz/gentax/internal/driver"
	"github.com/bmunoz/gentax/internal/expense"
	"github.com/bmunoz/gentax/internal/taxi"
)

// errorResponse is the standard JSON error body returned by all API errors.
type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// WriteError writes a JSON error response with the given status, human-readable
// message and machine-readable code.
func WriteError(w http.ResponseWriter, status int, msg, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: msg, Code: code})
}

// DomainError maps sentinel errors from domain packages to the appropriate
// HTTP status and machine code, then writes the error response.
//
// Mapping:
//
//	taxi.ErrNotFound / driver.ErrNotFound / expense.ErrNotFound → 404 not_found
//	taxi.ErrDuplicatePlate                                      → 409 duplicate_plate
//	taxi.ErrInvalidYear                                         → 422 invalid_year
//	driver.ErrInvalidInput                                      → 422 invalid_input
//	driver.ErrDuplicateTelegram                                 → 409 duplicate_telegram
//	driver.ErrLinkTokenExpired                                  → 422 link_token_expired
//	driver.ErrLinkTokenUsed                                     → 422 link_token_used
//	expense.ErrReceiptRequired                                  → 422 receipt_required
//	expense.ErrInvalidTransition                                → 409 invalid_transition
//	default                                                     → 500 internal_error
func DomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, taxi.ErrNotFound),
		errors.Is(err, driver.ErrNotFound),
		errors.Is(err, expense.ErrNotFound):
		WriteError(w, http.StatusNotFound, err.Error(), "not_found")

	case errors.Is(err, taxi.ErrDuplicatePlate):
		WriteError(w, http.StatusConflict, err.Error(), "duplicate_plate")

	case errors.Is(err, taxi.ErrInvalidYear):
		WriteError(w, http.StatusUnprocessableEntity, err.Error(), "invalid_year")

	case errors.Is(err, driver.ErrInvalidInput):
		WriteError(w, http.StatusUnprocessableEntity, err.Error(), "invalid_input")

	case errors.Is(err, driver.ErrDuplicateTelegram):
		WriteError(w, http.StatusConflict, err.Error(), "duplicate_telegram")

	case errors.Is(err, driver.ErrLinkTokenExpired):
		WriteError(w, http.StatusUnprocessableEntity, err.Error(), "link_token_expired")

	case errors.Is(err, driver.ErrLinkTokenUsed):
		WriteError(w, http.StatusUnprocessableEntity, err.Error(), "link_token_used")

	case errors.Is(err, expense.ErrReceiptRequired):
		WriteError(w, http.StatusUnprocessableEntity, err.Error(), "receipt_required")

	case errors.Is(err, expense.ErrInvalidTransition):
		WriteError(w, http.StatusConflict, err.Error(), "invalid_transition")

	default:
		WriteError(w, http.StatusInternalServerError, "internal server error", "internal_error")
	}
}
