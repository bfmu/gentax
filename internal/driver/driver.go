// Package driver contains the Driver domain entity, Repository interface, Service interface, and sentinel errors.
package driver

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors for the driver domain.
var (
	ErrNotFound          = errors.New("driver not found")
	ErrInvalidInput      = errors.New("invalid input")
	ErrDuplicateTelegram = errors.New("telegram ID already linked to another driver")
	ErrLinkTokenExpired  = errors.New("link token expired")
	ErrLinkTokenUsed     = errors.New("link token already used")
	ErrAlreadyAssigned   = errors.New("driver already has an active taxi assignment")
)

// Driver represents a driver registered to an owner.
type Driver struct {
	ID                 uuid.UUID  `json:"id"`
	OwnerID            uuid.UUID  `json:"owner_id"`
	TelegramID         *int64     `json:"telegram_id"` // nil until linked via bot
	FullName           string     `json:"full_name"`
	Phone              string     `json:"phone"`
	Active             bool       `json:"active"`
	LinkToken          *string    `json:"-"`
	LinkTokenExpiresAt *time.Time `json:"-"`
	LinkTokenUsed      bool       `json:"-"`
	CreatedAt          time.Time  `json:"created_at"`
}

// Assignment represents a driver-taxi assignment record.
type Assignment struct {
	ID           uuid.UUID
	DriverID     uuid.UUID
	TaxiID       uuid.UUID
	AssignedAt   time.Time
	UnassignedAt *time.Time
}

// CreateInput holds the data required to create a new driver.
type CreateInput struct {
	OwnerID  uuid.UUID
	FullName string
	Phone    string
}

// Repository defines the persistence contract for drivers.
// Every method that touches owner-scoped data MUST filter by ownerID.
type Repository interface {
	Create(ctx context.Context, input CreateInput) (*Driver, error)
	GetByID(ctx context.Context, id, ownerID uuid.UUID) (*Driver, error)
	GetByTelegramID(ctx context.Context, telegramID int64) (*Driver, error)
	GetByLinkToken(ctx context.Context, token string) (*Driver, error)
	List(ctx context.Context, ownerID uuid.UUID) ([]*Driver, error)
	SetActive(ctx context.Context, id, ownerID uuid.UUID, active bool) error
	SetLinkToken(ctx context.Context, driverID uuid.UUID, token string, expiresAt time.Time) error
	MarkLinkTokenUsed(ctx context.Context, driverID uuid.UUID) error
	SetTelegramID(ctx context.Context, driverID uuid.UUID, telegramID int64) error
	GetActiveAssignment(ctx context.Context, driverID uuid.UUID) (*Assignment, error)
	CreateAssignment(ctx context.Context, driverID, taxiID uuid.UUID) (*Assignment, error)
	UnassignDriver(ctx context.Context, driverID uuid.UUID) error
}

// Service exposes business operations on drivers.
type Service interface {
	Create(ctx context.Context, input CreateInput) (*Driver, error)
	GenerateLinkToken(ctx context.Context, driverID, ownerID uuid.UUID) (string, error)
	LinkTelegramID(ctx context.Context, token string, telegramID int64) error
	Deactivate(ctx context.Context, id, ownerID uuid.UUID) error
	List(ctx context.Context, ownerID uuid.UUID) ([]*Driver, error)
	AssignTaxi(ctx context.Context, driverID, taxiID, ownerID uuid.UUID) error
	UnassignTaxi(ctx context.Context, driverID, ownerID uuid.UUID) error
}
