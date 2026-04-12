// Package expense contains the Expense domain entity, Repository interface, Service interface, and sentinel errors.
package expense

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Status represents the lifecycle state of an expense.
type Status string

const (
	StatusPending       Status = "pending"
	StatusConfirmed     Status = "confirmed"      // driver-confirmed, awaiting admin approval
	StatusNeedsEvidence Status = "needs_evidence" // owner requested additional evidence from driver
	StatusApproved      Status = "approved"
	StatusRejected      Status = "rejected"
)

// validTransitions defines the allowed state machine moves.
// Terminal states (approved, rejected) have no outgoing transitions.
var validTransitions = map[Status][]Status{
	StatusPending:       {StatusConfirmed},
	StatusConfirmed:     {StatusApproved, StatusRejected, StatusNeedsEvidence},
	StatusNeedsEvidence: {StatusConfirmed, StatusRejected},
	StatusApproved:      {},
	StatusRejected:      {},
}

// Sentinel errors for the expense domain.
var (
	ErrNotFound                = errors.New("expense not found")
	ErrReceiptRequired         = errors.New("receipt is required for every expense")
	ErrInvalidTransition       = errors.New("invalid status transition")
	ErrEvidenceNotAllowed      = errors.New("expense is not in a state that allows evidence request")
	ErrEvidenceMessageRequired = errors.New("evidence request message is required")
	ErrCategoryNotFound        = errors.New("category not found")
	ErrCategoryInUse           = errors.New("category is in use by existing expenses")
	ErrCategoryNameRequired    = errors.New("category name is required")
	ErrCategoryDuplicate       = errors.New("category name already exists")
)

// Expense represents a taxi fleet expense submitted by a driver.
type Expense struct {
	ID              uuid.UUID        `json:"id"`
	OwnerID         uuid.UUID        `json:"owner_id"`
	DriverID        uuid.UUID        `json:"driver_id"`
	TaxiID          uuid.UUID        `json:"taxi_id"`
	CategoryID      uuid.UUID        `json:"category_id"`
	ReceiptID       uuid.UUID        `json:"receipt_id"`
	Amount          *decimal.Decimal `json:"amount"` // may be nil until OCR or manual entry
	ExpenseDate     *time.Time       `json:"expense_date"`
	Notes           string           `json:"notes"`
	Status          Status           `json:"status"`
	RejectionReason string           `json:"rejection_reason"`
	ReviewedBy      *uuid.UUID       `json:"reviewed_by"`
	ReviewedAt      *time.Time       `json:"reviewed_at"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	// Populated by List queries via JOIN with drivers, taxis, expense_categories, and receipts tables.
	DriverName     string  `json:"driver_name"`
	TaxiPlate      string  `json:"taxi_plate"`
	CategoryName   string  `json:"category_name"`
	ReceiptImageURL string `json:"receipt_image_url"`
	OCRRaw         *string `json:"ocr_raw,omitempty"`
}

// CreateInput holds the data required to create a new expense.
// OwnerID MUST come from the service layer (JWT claims), never from client input.
type CreateInput struct {
	OwnerID    uuid.UUID
	DriverID   uuid.UUID
	TaxiID     uuid.UUID
	CategoryID uuid.UUID
	ReceiptID  uuid.UUID // required; zero value → ErrReceiptRequired
	Notes      string
}

// ListFilter defines optional filters for listing expenses.
// OwnerID is always required for multi-tenant isolation.
type ListFilter struct {
	OwnerID    uuid.UUID  // required
	DriverID   *uuid.UUID
	TaxiID     *uuid.UUID
	CategoryID *uuid.UUID
	Status     *Status
	DateFrom   *time.Time
	DateTo     *time.Time
	Limit      int // default 20, max 100
	Offset     int
}

// TaxiSummary is one row in the per-taxi expense aggregate report.
type TaxiSummary struct {
	TaxiID    uuid.UUID       `json:"taxi_id"`
	TaxiPlate string          `json:"taxi_plate"`
	Total     decimal.Decimal `json:"total"`
	Count     int             `json:"count"`
}

// DriverSummary is one row in the per-driver expense aggregate report.
type DriverSummary struct {
	DriverID   uuid.UUID       `json:"driver_id"`
	DriverName string          `json:"driver_name"`
	Total      decimal.Decimal `json:"total"`
	Count      int             `json:"count"`
}

// CategorySummary is one row in the per-category expense aggregate report.
type CategorySummary struct {
	CategoryID   uuid.UUID
	CategoryName string
	Total        decimal.Decimal
	Count        int
}

// ExpenseCategory is a lightweight view of an expense category used in UIs.
type ExpenseCategory struct {
	ID        uuid.UUID `json:"id"`
	OwnerID   uuid.UUID `json:"owner_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Repository defines the persistence contract for expenses.
// Every method that filters records MUST include owner_id to enforce multi-tenant isolation.
type Repository interface {
	Create(ctx context.Context, input CreateInput) (*Expense, error)
	GetByID(ctx context.Context, id, ownerID uuid.UUID) (*Expense, error)
	List(ctx context.Context, filter ListFilter) ([]*Expense, error)
	UpdateStatus(ctx context.Context, id, ownerID uuid.UUID, status Status, reviewedBy *uuid.UUID, rejectionReason string) error
	UpdateAmount(ctx context.Context, id uuid.UUID, amount decimal.Decimal) error
	UpdateReceiptID(ctx context.Context, id uuid.UUID, receiptID uuid.UUID) error
	ListCategories(ctx context.Context, ownerID uuid.UUID) ([]*ExpenseCategory, error)
	CreateCategory(ctx context.Context, ownerID uuid.UUID, name string) (*ExpenseCategory, error)
	DeleteCategory(ctx context.Context, id, ownerID uuid.UUID) error
	SeedDefaultCategories(ctx context.Context, ownerID uuid.UUID) error
	SumByTaxi(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*TaxiSummary, error)
	SumByDriver(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*DriverSummary, error)
	SumByCategory(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*CategorySummary, error)
}

// Service exposes business operations on expenses.
type Service interface {
	Create(ctx context.Context, input CreateInput) (*Expense, error)
	Confirm(ctx context.Context, id, driverID uuid.UUID) error
	Approve(ctx context.Context, id, ownerID uuid.UUID) error
	Reject(ctx context.Context, id, ownerID uuid.UUID, reason string) error
	RequestEvidence(ctx context.Context, id, ownerID uuid.UUID, message string) error
	SubmitEvidence(ctx context.Context, id, driverID uuid.UUID, receiptID uuid.UUID) error
	List(ctx context.Context, filter ListFilter) ([]*Expense, error)
	GetByID(ctx context.Context, id, ownerID uuid.UUID) (*Expense, error)
	UpdateAmount(ctx context.Context, id uuid.UUID, amount decimal.Decimal) error
	ListCategories(ctx context.Context, ownerID uuid.UUID) ([]*ExpenseCategory, error)
	CreateCategory(ctx context.Context, ownerID uuid.UUID, name string) (*ExpenseCategory, error)
	DeleteCategory(ctx context.Context, id, ownerID uuid.UUID) error
	SeedDefaultCategories(ctx context.Context, ownerID uuid.UUID) error
	SumByTaxi(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*TaxiSummary, error)
	SumByDriver(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*DriverSummary, error)
	SumByCategory(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*CategorySummary, error)
}
