package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/bmunoz/gentax/internal/auth"
	"github.com/bmunoz/gentax/internal/driver"
	mw "github.com/bmunoz/gentax/internal/httpapi/middleware"
)

// DriverFinder is the minimal interface needed by AuthHandler to look up a driver
// by Telegram ID for the bootstrap authentication endpoint.
// driver.Repository satisfies this interface.
type DriverFinder interface {
	GetByTelegramID(ctx context.Context, telegramID int64) (*driver.Driver, error)
}

// TelegramAuthRequest is the request body for POST /auth/telegram.
// Only telegram_id is accepted; owner_id must NOT be supplied by clients (REQ-FRD-04).
type TelegramAuthRequest struct {
	TelegramID int64 `json:"telegram_id"`
}

// telegramAuthResponse is the response body for POST /auth/telegram.
type telegramAuthResponse struct {
	Token string `json:"token"`
}

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	finder DriverFinder
	issuer auth.TokenIssuer
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(finder DriverFinder, issuer auth.TokenIssuer) *AuthHandler {
	return &AuthHandler{finder: finder, issuer: issuer}
}

// TelegramAuth handles POST /auth/telegram.
// It looks up the driver by telegram_id (globally unique across all owners) and issues
// a JWT with role=driver.  Returns 404 if no driver is linked to that telegram_id.
func (h *AuthHandler) TelegramAuth(w http.ResponseWriter, r *http.Request) {
	var req TelegramAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	if req.TelegramID == 0 {
		mw.WriteError(w, http.StatusUnprocessableEntity, "telegram_id is required", "invalid_input")
		return
	}

	drv, err := h.finder.GetByTelegramID(r.Context(), req.TelegramID)
	if err != nil {
		mw.DomainError(w, err)
		return
	}

	claims := auth.Claims{
		UserID:   drv.ID,
		Role:     auth.RoleDriver,
		OwnerID:  drv.OwnerID,
		DriverID: &drv.ID,
	}

	token, err := h.issuer.Issue(claims, time.Hour)
	if err != nil {
		mw.WriteError(w, http.StatusInternalServerError, "failed to issue token", "internal_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(telegramAuthResponse{Token: token})
}
