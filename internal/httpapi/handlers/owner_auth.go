package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bmunoz/gentax/internal/auth"
	"github.com/bmunoz/gentax/internal/expense"
	"github.com/bmunoz/gentax/internal/owner"
	mw "github.com/bmunoz/gentax/internal/httpapi/middleware"
)

// OwnerAuthService is the interface needed by OwnerAuthHandler.
// owner.Service satisfies this interface.
type OwnerAuthService interface {
	Authenticate(ctx context.Context, email, password string) (*owner.Owner, error)
	Create(ctx context.Context, name, email, password string) (*owner.Owner, error)
	Count(ctx context.Context) (int, error)
}

// OwnerAuthHandler handles owner authentication endpoints.
type OwnerAuthHandler struct {
	svc             OwnerAuthService
	issuer          auth.TokenIssuer
	bootstrapSecret string
	expenseSvc      expense.Service
}

// NewOwnerAuthHandler constructs an OwnerAuthHandler.
func NewOwnerAuthHandler(svc OwnerAuthService, issuer auth.TokenIssuer, bootstrapSecret string) *OwnerAuthHandler {
	return &OwnerAuthHandler{svc: svc, issuer: issuer, bootstrapSecret: bootstrapSecret}
}

// WithExpenseService sets the optional expense service used to seed default categories on registration.
func (h *OwnerAuthHandler) WithExpenseService(svc expense.Service) *OwnerAuthHandler {
	h.expenseSvc = svc
	return h
}

type ownerLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ownerBootstrapRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ownerAuthResponse struct {
	Token string `json:"token"`
}

// Login handles POST /auth/owner/login.
// Returns 200 {token} on success, 401 on invalid credentials.
func (h *OwnerAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req ownerLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	o, err := h.svc.Authenticate(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, owner.ErrInvalidCredentials) {
			mw.WriteError(w, http.StatusUnauthorized, "invalid email or password", "invalid_credentials")
			return
		}
		mw.WriteError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}

	token, err := h.issueOwnerToken(o)
	if err != nil {
		mw.WriteError(w, http.StatusInternalServerError, "failed to issue token", "internal_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ownerAuthResponse{Token: token})
}

// Bootstrap handles POST /auth/owner/bootstrap.
// Requires X-Bootstrap-Secret header matching BOOTSTRAP_SECRET env var.
// Returns 201 {token} on success, 403 on wrong secret, 409 if owner already exists.
func (h *OwnerAuthHandler) Bootstrap(w http.ResponseWriter, r *http.Request) {
	if h.bootstrapSecret == "" || r.Header.Get("X-Bootstrap-Secret") != h.bootstrapSecret {
		mw.WriteError(w, http.StatusForbidden, "invalid bootstrap secret", "forbidden")
		return
	}

	count, err := h.svc.Count(r.Context())
	if err != nil {
		mw.WriteError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	if count > 0 {
		mw.WriteError(w, http.StatusConflict, "owner already exists", "owner_exists")
		return
	}

	var req ownerBootstrapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	o, err := h.svc.Create(r.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, owner.ErrDuplicateEmail) {
			mw.WriteError(w, http.StatusConflict, "email already registered", "duplicate_email")
			return
		}
		mw.WriteError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}

	token, err := h.issueOwnerToken(o)
	if err != nil {
		mw.WriteError(w, http.StatusInternalServerError, "failed to issue token", "internal_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(ownerAuthResponse{Token: token})
}

type ownerRegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Register handles POST /auth/owner/register.
// Returns 201 {"message":"registered"} on success.
func (h *OwnerAuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req ownerRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)
	req.Password = strings.TrimSpace(req.Password)

	if req.Name == "" || req.Email == "" || req.Password == "" {
		mw.WriteError(w, http.StatusBadRequest, "name, email and password are required", "validation_error")
		return
	}

	if len(req.Password) < 8 {
		mw.WriteError(w, http.StatusBadRequest, "password must be at least 8 characters", "password_too_short")
		return
	}

	newOwner, err := h.svc.Create(r.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, owner.ErrDuplicateEmail) {
			mw.WriteError(w, http.StatusConflict, "email already registered", "duplicate_email")
			return
		}
		mw.WriteError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}

	if h.expenseSvc != nil {
		if err := h.expenseSvc.SeedDefaultCategories(r.Context(), newOwner.ID); err != nil {
			slog.Warn("failed to seed default categories for new owner", "owner_id", newOwner.ID, "err", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "registered"})
}

func (h *OwnerAuthHandler) issueOwnerToken(o *owner.Owner) (string, error) {
	claims := auth.Claims{
		UserID:  o.ID,
		Role:    auth.RoleAdmin,
		OwnerID: o.ID,
	}
	return h.issuer.Issue(claims, 8*time.Hour)
}
