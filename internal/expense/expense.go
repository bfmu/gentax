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
	StatusPending   Status = "pending"
	StatusConfirmed Status = "confirmed" // driver-confirmed, awaiting admin approval
	StatusApproved  Status = "approved"
	StatusRejected  Status = "rejected"
)

// validTransitions defines the allowed state machine moves.
// Terminal states (approved, rejected) have no outgoing transitions.
var validTransitions = map[Status][]Status{
	StatusPending:   {StatusConfirmed},
	StatusConfirmed: {StatusApproved, StatusRejected},
	StatusApproved:  {},
	StatusRejected:  {},
}

// Sentinel errors for the expense domain.
var (
	ErrNotFound          = errors.New("expense not found")
	ErrReceiptRequired   = errors.New("receipt is required for every expense")
	ErrInvalidTransition = errors.New("invalid status transition")
)

// Expense represents a taxi fleet expense submitted by a driver.
type Expense struct {
	ID              uuid.UUID
	OwnerID         uuid.UUID
	DriverID        uuid.UUID
	TaxiID          uuid.UUID
	CategoryID      uuid.UUID
	ReceiptID       uuid.UUID        // NOT NULL — fraud prevention (REQ-FRD-01)
	Amount          *decimal.Decimal // may be nil until OCR or manual entry
	ExpenseDate     *time.Time
	Notes           string
	Status          Status
	RejectionReason string
	ReviewedBy      *uuid.UUID
	ReviewedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
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
	TaxiID    uuid.UUID
	TaxiPlate string
	Total     decimal.Decimal
	Count     int
}

// DriverSummary is one row in the per-driver expense aggregate report.
type DriverSummary struct {
	DriverID   uuid.UUID
	DriverName string
	Total      decimal.Decimal
	Count      int
}

// CategorySummary is one row in the per-category expense aggregate report.
type CategorySummary struct {
	CategoryID   uuid.UUID
	CategoryName string
	Total        decimal.Decimal
	Count        int
}

// Repository defines the persistence contract for expenses.
// Every method that filters records MUST include owner_id to enforce multi-tenant isolation.
type Repository interface {
	Create(ctx context.Context, input CreateInput) (*Expense, error)
	GetByID(ctx context.Context, id, ownerID uuid.UUID) (*Expense, error)
	List(ctx context.Context, filter ListFilter) ([]*Expense, error)
	UpdateStatus(ctx context.Context, id, ownerID uuid.UUID, status Status, reviewedBy *uuid.UUID, rejectionReason string) error
	UpdateAmount(ctx context.Context, id uuid.UUID, amount decimal.Decimal) error
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
	List(ctx context.Context, filter ListFilter) ([]*Expense, error)
	GetByID(ctx context.Context, id, ownerID uuid.UUID) (*Expense, error)
	UpdateAmount(ctx context.Context, id uuid.UUID, amount decimal.Decimal) error
	SumByTaxi(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*TaxiSummary, error)
	SumByDriver(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*DriverSummary, error)
	SumByCategory(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*CategorySummary, error)
}
