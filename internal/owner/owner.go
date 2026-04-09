// Package owner provides the Owner entity, repository interface, service interface, and sentinel errors.
package owner

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Owner represents a fleet owner.
type Owner struct {
	ID           uuid.UUID
	Name         string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// Sentinel errors.
var (
	ErrNotFound           = errors.New("owner not found")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrDuplicateEmail     = errors.New("email already registered")
)

// Repository defines storage operations for owners.
type Repository interface {
	Create(ctx context.Context, name, email, passwordHash string) (*Owner, error)
	GetByEmail(ctx context.Context, email string) (*Owner, error)
	Count(ctx context.Context) (int, error)
}

// Service defines business operations for owners.
type Service interface {
	Create(ctx context.Context, name, email, plainPassword string) (*Owner, error)
	Authenticate(ctx context.Context, email, plainPassword string) (*Owner, error)
	Count(ctx context.Context) (int, error)
}
