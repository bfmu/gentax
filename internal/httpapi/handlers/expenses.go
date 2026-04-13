package handlers

import (
	"context"
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

// EvidenceNotifier sends Telegram notifications to drivers for expense lifecycle events.
// Implementations must be nil-safe — the handler checks for nil before calling.
type EvidenceNotifier interface {
	NotifyEvidenceRequest(ctx context.Context, expenseID uuid.UUID, message string) error
	NotifyRejection(ctx context.Context, expenseID uuid.UUID, reason string) error
}

// StorageReader downloads bytes from a storage URL.
// Implementations must support file:// URLs (by reading from the local filesystem)
// and may delegate cloud URLs to a cloud storage client.
type StorageReader interface {
	Download(ctx context.Context, url string) ([]byte, error)
}

// ExpenseHandler handles /expenses endpoints.
type ExpenseHandler struct {
	expenseSvc       expense.Service
	evidenceNotifier EvidenceNotifier
	storageReader    StorageReader
}

// NewExpenseHandler constructs an ExpenseHandler.
// The optional notifier is used to send Telegram notifications when evidence is requested.
// Pass nil to disable notifications.
func NewExpenseHandler(expenseSvc expense.Service, notifier ...EvidenceNotifier) *ExpenseHandler {
	h := &ExpenseHandler{expenseSvc: expenseSvc}
	if len(notifier) > 0 {
		h.evidenceNotifier = notifier[0]
	}
	return h
}

// WithEvidenceNotifier sets the optional notifier for evidence requests and returns the handler.
func (h *ExpenseHandler) WithEvidenceNotifier(n EvidenceNotifier) *ExpenseHandler {
	h.evidenceNotifier = n
	return h
}

// WithStorageReader sets the optional storage reader for receipt proxy requests.
func (h *ExpenseHandler) WithStorageReader(r StorageReader) *ExpenseHandler {
	h.storageReader = r
	return h
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

	if h.evidenceNotifier != nil {
		go func() { _ = h.evidenceNotifier.NotifyRejection(context.Background(), id, req.Reason) }()
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

// requestEvidenceRequest is the body for PATCH /expenses/{id}/request-evidence.
type requestEvidenceRequest struct {
	Message string `json:"message"`
}

// RequestEvidence handles PATCH /expenses/{id}/request-evidence (role=admin).
// Transitions the expense to needs_evidence and optionally notifies the driver via Telegram.
func (h *ExpenseHandler) RequestEvidence(w http.ResponseWriter, r *http.Request) {
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

	var req requestEvidenceRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.expenseSvc.RequestEvidence(r.Context(), id, claims.OwnerID, req.Message); err != nil {
		mw.DomainError(w, err)
		return
	}

	if h.evidenceNotifier != nil {
		go func() { _ = h.evidenceNotifier.NotifyEvidenceRequest(context.Background(), id, req.Message) }()
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "needs_evidence"})
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

	if statusVals := q["status"]; len(statusVals) > 0 {
		for _, v := range statusVals {
			filter.Statuses = append(filter.Statuses, expense.Status(v))
		}
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

// addAttachmentRequest is the request body for POST /expenses/{id}/attachments.
type addAttachmentRequest struct {
	ReceiptID uuid.UUID `json:"receipt_id"`
	Label     string    `json:"label"`
}

// ListAttachments handles GET /expenses/{id}/attachments (role=admin).
// Returns all attachments for the expense, including storage URLs.
func (h *ExpenseHandler) ListAttachments(w http.ResponseWriter, r *http.Request) {
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

	attachments, err := h.expenseSvc.ListAttachments(r.Context(), id, claims.OwnerID)
	if err != nil {
		mw.DomainError(w, err)
		return
	}

	// Return empty array rather than null when there are no attachments.
	if attachments == nil {
		attachments = []expense.Attachment{}
	}
	writeJSON(w, http.StatusOK, attachments)
}

// AddAttachment handles POST /expenses/{id}/attachments (role=driver).
// Attaches an already-uploaded receipt to the expense as additional evidence.
func (h *ExpenseHandler) AddAttachment(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	if claims.DriverID == nil {
		mw.WriteError(w, http.StatusForbidden, "forbidden: not a driver", "forbidden")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid expense id", "bad_request")
		return
	}

	var req addAttachmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		mw.WriteError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	if req.ReceiptID == uuid.Nil {
		mw.DomainError(w, expense.ErrReceiptRequired)
		return
	}

	if err := h.expenseSvc.AddAttachment(r.Context(), id, *claims.DriverID, req.ReceiptID, req.Label); err != nil {
		mw.DomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "attached"})
}

// ReceiptProxy handles GET /expenses/{id}/receipt.
// It fetches the storage URL from the service layer, then reads the bytes (via StorageReader
// for file:// paths or direct HTTP for cloud URLs) and pipes them to the client.
func (h *ExpenseHandler) ReceiptProxy(w http.ResponseWriter, r *http.Request) {
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

	storageURL, err := h.expenseSvc.GetReceiptStorageURL(r.Context(), id, claims.OwnerID)
	if err != nil {
		mw.DomainError(w, err)
		return
	}

	var data []byte
	if h.storageReader != nil {
		data, err = h.storageReader.Download(r.Context(), storageURL)
		if err != nil {
			mw.WriteError(w, http.StatusBadGateway, "failed to fetch receipt", "storage_error")
			return
		}
	} else {
		mw.WriteError(w, http.StatusServiceUnavailable, "storage reader not configured", "not_configured")
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
