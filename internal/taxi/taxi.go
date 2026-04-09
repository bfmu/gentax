// Package taxi contains the Taxi domain entity, Repository interface, Service interface, and sentinel errors.
package taxi

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors for the taxi domain.
var (
	ErrNotFound       = errors.New("taxi not found")
	ErrDuplicatePlate = errors.New("plate already exists for this owner")
	ErrInvalidYear    = errors.New("taxi year out of valid range")
)

// Taxi represents a vehicle registered to an owner.
type Taxi struct {
	ID        uuid.UUID
	OwnerID   uuid.UUID
	Plate     string
	Model     string
	Year      int
	Active    bool
	CreatedAt time.Time
}

// CreateInput holds the data required to create a new taxi.
type CreateInput struct {
	OwnerID uuid.UUID
	Plate   string
	Model   string
	Year    int
}

// Repository defines the persistence contract for taxis.
// Every method MUST filter by ownerID to enforce multi-tenant isolation.
type Repository interface {
	Create(ctx context.Context, input CreateInput) (*Taxi, error)
	GetByID(ctx context.Context, id, ownerID uuid.UUID) (*Taxi, error)
	List(ctx context.Context, ownerID uuid.UUID) ([]*Taxi, error)
	SetActive(ctx context.Context, id, ownerID uuid.UUID, active bool) error
}

// Service exposes business operations on taxis.
type Service interface {
	Create(ctx context.Context, input CreateInput) (*Taxi, error)
	List(ctx context.Context, ownerID uuid.UUID) ([]*Taxi, error)
	Deactivate(ctx context.Context, id, ownerID uuid.UUID) error
}
