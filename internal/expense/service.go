package expense

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)



const (
	defaultLimit = 20
	maxLimit     = 100
)

type service struct {
	repo Repository
}

// NewService constructs an expense Service backed by the given Repository.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// canTransition returns true if transitioning from → to is permitted by the state machine.
func canTransition(from, to Status) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// Create validates input and creates a new expense with status=pending.
// REQ-FRD-01: ReceiptID must not be nil (zero UUID).
// REQ-FRD-04: OwnerID comes from service input (JWT claims), never from request params.
func (s *service) Create(ctx context.Context, input CreateInput) (*Expense, error) {
	if input.ReceiptID == uuid.Nil {
		return nil, ErrReceiptRequired
	}
	return s.repo.Create(ctx, input)
}

// Confirm transitions a pending expense to confirmed (driver-side confirmation).
// REQ-OCR-05: driver confirms OCR data → status=confirmed.
func (s *service) Confirm(ctx context.Context, id, driverID uuid.UUID) error {
	// GetByID with driverID as ownerID placeholder — we do a manual driverID check below.
	// Repository GetByID requires ownerID; for driver-side confirm we look up by id only via a
	// zero ownerID query then verify driverID ourselves.
	// Design choice: use uuid.Nil as ownerID sentinel and let repo return the record,
	// then verify driverID matches. However, the Repository interface requires ownerID scoping.
	// To keep isolation intact we pass uuid.Nil and verify in the service.
	// The repo must return the expense regardless of ownerID when ownerID==uuid.Nil.
	// Better: call repo.GetByID with the expense's actual ownerID is unknown here.
	// Solution: Repository has GetByID(id, ownerID). For Confirm we need to fetch by id+driverID.
	// We model this by calling GetByID with uuid.Nil — the repo impl must handle this by
	// skipping the owner_id filter when ownerID is uuid.Nil. Alternatively we add a
	// GetByIDForDriver method. Per the spec, Confirm takes driverID not ownerID.
	//
	// Implementation decision: call GetByID(id, uuid.Nil) — repository skips owner filter when nil.
	// This is documented and only used internally for the Confirm path.
	exp, err := s.repo.GetByID(ctx, id, uuid.Nil)
	if err != nil {
		return err
	}

	// Verify the requesting driver owns this expense.
	if exp.DriverID != driverID {
		return ErrNotFound
	}

	if !canTransition(exp.Status, StatusConfirmed) {
		return ErrInvalidTransition
	}

	return s.repo.UpdateStatus(ctx, id, exp.OwnerID, StatusConfirmed, nil, "")
}

// Approve transitions a confirmed expense to approved.
// REQ-APR-02: only confirmed expenses may be approved.
// reviewedBy is set to ownerID; reviewedAt is set to now() by the repository.
func (s *service) Approve(ctx context.Context, id, ownerID uuid.UUID) error {
	exp, err := s.repo.GetByID(ctx, id, ownerID)
	if err != nil {
		return err
	}

	if !canTransition(exp.Status, StatusApproved) {
		return ErrInvalidTransition
	}

	return s.repo.UpdateStatus(ctx, id, ownerID, StatusApproved, &ownerID, "")
}

// Reject transitions a confirmed expense to rejected with an optional reason.
// REQ-APR-03: only confirmed expenses may be rejected.
func (s *service) Reject(ctx context.Context, id, ownerID uuid.UUID, reason string) error {
	exp, err := s.repo.GetByID(ctx, id, ownerID)
	if err != nil {
		return err
	}

	if !canTransition(exp.Status, StatusRejected) {
		return ErrInvalidTransition
	}

	return s.repo.UpdateStatus(ctx, id, ownerID, StatusRejected, &ownerID, reason)
}

// RequestEvidence transitions a confirmed expense to needs_evidence,
// storing the owner's message in the rejection_reason column.
// Idempotent: if already in needs_evidence, updates the message and returns nil.
// REQ-EVD-02: message must be non-empty; expense must be in confirmed or needs_evidence state.
func (s *service) RequestEvidence(ctx context.Context, id, ownerID uuid.UUID, message string) error {
	if strings.TrimSpace(message) == "" {
		return ErrEvidenceMessageRequired
	}
	exp, err := s.repo.GetByID(ctx, id, ownerID)
	if err != nil {
		return err
	}
	// Idempotent re-request: already in needs_evidence → just refresh the message.
	if exp.Status != StatusNeedsEvidence && !canTransition(exp.Status, StatusNeedsEvidence) {
		return ErrInvalidTransition
	}
	return s.repo.UpdateStatus(ctx, id, ownerID, StatusNeedsEvidence, nil, message)
}

// SubmitEvidence transitions a needs_evidence expense back to confirmed after a driver uploads a new receipt.
// The driver must match the expense's driverID for security.
func (s *service) SubmitEvidence(ctx context.Context, id, driverID uuid.UUID, receiptID uuid.UUID) error {
	exp, err := s.repo.GetByID(ctx, id, uuid.Nil) // driver path: skip owner filter
	if err != nil {
		return err
	}
	if exp.DriverID != driverID {
		return ErrNotFound
	}
	if exp.Status != StatusNeedsEvidence {
		return ErrInvalidTransition
	}
	if err := s.repo.UpdateReceiptID(ctx, id, receiptID); err != nil {
		return err
	}
	return s.repo.UpdateStatus(ctx, id, exp.OwnerID, StatusConfirmed, nil, "")
}

// List returns expenses matching the filter, scoped to the owner.
// Limit is clamped to [1, 100]; zero becomes defaultLimit (20).
func (s *service) List(ctx context.Context, filter ListFilter) ([]*Expense, error) {
	if filter.Limit <= 0 {
		filter.Limit = defaultLimit
	} else if filter.Limit > maxLimit {
		filter.Limit = maxLimit
	}
	return s.repo.List(ctx, filter)
}

// GetByID returns an expense by ID, scoped to the owner.
// REQ-APR-04, REQ-TNT-03: wrong owner → ErrNotFound (not 403).
func (s *service) GetByID(ctx context.Context, id, ownerID uuid.UUID) (*Expense, error) {
	return s.repo.GetByID(ctx, id, ownerID)
}

// UpdateAmount updates the amount for an expense (used after OCR or manual correction).
func (s *service) UpdateAmount(ctx context.Context, id uuid.UUID, amount decimal.Decimal) error {
	return s.repo.UpdateAmount(ctx, id, amount)
}

// UpdateAmountByReceiptID looks up an expense by its receipt ID, then updates its amount.
// Used by the OCR processor to set the extracted total without knowing the expense ID.
func (s *service) UpdateAmountByReceiptID(ctx context.Context, receiptID uuid.UUID, amount decimal.Decimal) error {
	exp, err := s.repo.GetByReceiptID(ctx, receiptID)
	if err != nil {
		return err
	}
	return s.repo.UpdateAmount(ctx, exp.ID, amount)
}

// GetReceiptStorageURL returns the storage URL for the receipt associated with an expense.
// Scoped to ownerID for multi-tenant isolation.
func (s *service) GetReceiptStorageURL(ctx context.Context, id, ownerID uuid.UUID) (string, error) {
	return s.repo.GetReceiptStorageURL(ctx, id, ownerID)
}

// AttachOptionalEvidence attaches a new receipt to an existing expense without changing its status.
// Only the driver who owns the expense may attach evidence.
func (s *service) AttachOptionalEvidence(ctx context.Context, expenseID, driverID, receiptID uuid.UUID) error {
	exp, err := s.repo.GetByID(ctx, expenseID, uuid.Nil) // driver path: skip owner filter
	if err != nil {
		return err
	}
	if exp.DriverID != driverID {
		return ErrNotFound
	}
	return s.repo.UpdateReceiptID(ctx, expenseID, receiptID)
}

// ListCategories returns all expense categories for the given owner.
func (s *service) ListCategories(ctx context.Context, ownerID uuid.UUID) ([]*ExpenseCategory, error) {
	return s.repo.ListCategories(ctx, ownerID)
}

// CreateCategory creates a new expense category for the given owner.
// Returns ErrCategoryNameRequired if name is blank.
func (s *service) CreateCategory(ctx context.Context, ownerID uuid.UUID, name string) (*ExpenseCategory, error) {
	if strings.TrimSpace(name) == "" {
		return nil, ErrCategoryNameRequired
	}
	return s.repo.CreateCategory(ctx, ownerID, strings.TrimSpace(name))
}

// DeleteCategory removes an expense category scoped to the owner.
func (s *service) DeleteCategory(ctx context.Context, id, ownerID uuid.UUID) error {
	return s.repo.DeleteCategory(ctx, id, ownerID)
}

// SeedDefaultCategories seeds the default expense categories for the given owner.
func (s *service) SeedDefaultCategories(ctx context.Context, ownerID uuid.UUID) error {
	return s.repo.SeedDefaultCategories(ctx, ownerID)
}

// SumByTaxi returns aggregate totals per taxi for approved expenses in the date range.
func (s *service) SumByTaxi(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*TaxiSummary, error) {
	return s.repo.SumByTaxi(ctx, ownerID, from, to)
}

// SumByDriver returns aggregate totals per driver for approved expenses in the date range.
func (s *service) SumByDriver(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*DriverSummary, error) {
	return s.repo.SumByDriver(ctx, ownerID, from, to)
}

// SumByCategory returns aggregate totals per category for approved expenses in the date range.
func (s *service) SumByCategory(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*CategorySummary, error) {
	return s.repo.SumByCategory(ctx, ownerID, from, to)
}

// AddAttachment attaches a receipt to an existing expense as additional evidence.
// Only the driver who owns the expense may attach evidence.
// Keeps backward-compat: does not change the expense's primary receipt_id.
func (s *service) AddAttachment(ctx context.Context, expenseID, driverID uuid.UUID, receiptID uuid.UUID, label string) error {
	exp, err := s.repo.GetByID(ctx, expenseID, uuid.Nil) // driver path: skip owner filter
	if err != nil {
		return err
	}
	if exp.DriverID != driverID {
		return ErrNotFound
	}
	_, err = s.repo.AddAttachment(ctx, expenseID, receiptID, label)
	return err
}

// ListAttachments returns all attachments for the given expense, scoped to ownerID.
func (s *service) ListAttachments(ctx context.Context, expenseID, ownerID uuid.UUID) ([]Attachment, error) {
	// Verify the expense belongs to the owner.
	if _, err := s.repo.GetByID(ctx, expenseID, ownerID); err != nil {
		return nil, err
	}
	return s.repo.ListAttachments(ctx, expenseID)
}
