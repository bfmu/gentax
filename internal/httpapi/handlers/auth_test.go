package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bmunoz/gentax/internal/auth"
	"github.com/bmunoz/gentax/internal/driver"
)

func TestAuthHandler_TelegramAuth_Success(t *testing.T) {
	ownerID := uuid.New()
	driverID := uuid.New()

	finder := &mockDriverFinder{}
	issuer := &mockTokenIssuer{}

	drv := &driver.Driver{
		ID:      driverID,
		OwnerID: ownerID,
	}
	finder.On("GetByTelegramID", matchAny, int64(123456)).Return(drv, nil)
	issuer.On("Issue", auth.Claims{
		UserID:   driverID,
		Role:     auth.RoleDriver,
		OwnerID:  ownerID,
		DriverID: &driverID,
	}, time.Hour).Return("tok.en.value", nil)

	h := NewAuthHandler(finder, issuer)
	body := `{"telegram_id": 123456}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/telegram", bytes.NewBufferString(body))

	h.TelegramAuth(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp telegramAuthResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "tok.en.value", resp.Token)
	finder.AssertExpectations(t)
	issuer.AssertExpectations(t)
}

func TestAuthHandler_TelegramAuth_NotFound(t *testing.T) {
	finder := &mockDriverFinder{}
	issuer := &mockTokenIssuer{}

	finder.On("GetByTelegramID", matchAny, int64(999)).Return(nil, driver.ErrNotFound)

	h := NewAuthHandler(finder, issuer)
	body := `{"telegram_id": 999}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/telegram", bytes.NewBufferString(body))

	h.TelegramAuth(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
	finder.AssertExpectations(t)
}

func TestAuthHandler_TelegramAuth_MissingTelegramID(t *testing.T) {
	finder := &mockDriverFinder{}
	issuer := &mockTokenIssuer{}

	h := NewAuthHandler(finder, issuer)
	body := `{"telegram_id": 0}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/telegram", bytes.NewBufferString(body))

	h.TelegramAuth(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
