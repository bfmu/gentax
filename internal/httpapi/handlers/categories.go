package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bmunoz/gentax/internal/auth"
	"github.com/bmunoz/gentax/internal/expense"
	mw "github.com/bmunoz/gentax/internal/httpapi/middleware"
)

// CategoryHandler handles /categories endpoints.
type CategoryHandler struct {
	expenseSvc expense.Service
}

// NewCategoryHandler constructs a CategoryHandler.
func NewCategoryHandler(svc expense.Service) *CategoryHandler {
	return &CategoryHandler{expenseSvc: svc}
}

// List handles GET /categories — returns all categories for the authenticated owner.
func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	cats, err := h.expenseSvc.ListCategories(r.Context(), claims.OwnerID)
	if err != nil {
		mw.DomainError(w, err)
		return
	}
	if cats == nil {
		cats = []*expense.ExpenseCategory{}
	}
	writeJSON(w, http.StatusOK, cats)
}

type createCategoryRequest struct {
	Name string `json:"name"`
}

// Create handles POST /categories — creates a new category for the authenticated owner.
func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var req createCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	cat, err := h.expenseSvc.CreateCategory(r.Context(), claims.OwnerID, req.Name)
	if err != nil {
		mw.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cat)
}

// Delete handles DELETE /categories/{id} — removes a category scoped to the authenticated owner.
func (h *CategoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid category id", "bad_request")
		return
	}
	if err := h.expenseSvc.DeleteCategory(r.Context(), id, claims.OwnerID); err != nil {
		mw.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
