package driver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	linkTokenBytes  = 32
	linkTokenExpiry = 24 * time.Hour
)

type service struct {
	repo Repository
}

// NewService constructs a driver Service backed by the given Repository.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// Create validates the input and persists a new driver.
// FullName must not be blank. owner_id is taken exclusively from input.OwnerID.
func (s *service) Create(ctx context.Context, input CreateInput) (*Driver, error) {
	if strings.TrimSpace(input.FullName) == "" {
		return nil, ErrInvalidInput
	}
	return s.repo.Create(ctx, input)
}

// GenerateLinkToken generates a cryptographically random single-use token for the given driver,
// stores it with a 24-hour expiry, and returns the hex-encoded token string.
func (s *service) GenerateLinkToken(ctx context.Context, driverID, ownerID uuid.UUID) (string, error) {
	// Verify ownership before issuing token.
	if _, err := s.repo.GetByID(ctx, driverID, ownerID); err != nil {
		return "", err
	}

	b := make([]byte, linkTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	expiresAt := time.Now().Add(linkTokenExpiry)

	if err := s.repo.SetLinkToken(ctx, driverID, token, expiresAt); err != nil {
		return "", err
	}
	return token, nil
}

// LinkTelegramID validates the link token and, if valid, sets the telegram_id on the driver.
func (s *service) LinkTelegramID(ctx context.Context, token string, telegramID int64) error {
	drv, err := s.repo.GetByLinkToken(ctx, token)
	if err != nil {
		return err
	}

	if drv.LinkTokenExpiresAt == nil || time.Now().After(*drv.LinkTokenExpiresAt) {
		return ErrLinkTokenExpired
	}
	if drv.LinkTokenUsed {
		return ErrLinkTokenUsed
	}

	if err := s.repo.SetTelegramID(ctx, drv.ID, telegramID); err != nil {
		return err
	}
	return s.repo.MarkLinkTokenUsed(ctx, drv.ID)
}

// Deactivate soft-deletes a driver by setting active=false.
// It verifies ownership, unassigns any active taxi (ignoring not-found), then sets active=false.
func (s *service) Deactivate(ctx context.Context, id, ownerID uuid.UUID) error {
	if _, err := s.repo.GetByID(ctx, id, ownerID); err != nil {
		return err // propagates ErrNotFound for wrong owner
	}

	// Unassign active taxi; ignore ErrNotFound (driver may have no assignment).
	if err := s.repo.UnassignDriver(ctx, id); err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}

	return s.repo.SetActive(ctx, id, ownerID, false)
}

// List returns all drivers belonging to the given owner.
func (s *service) List(ctx context.Context, ownerID uuid.UUID) ([]*Driver, error) {
	return s.repo.List(ctx, ownerID)
}

// AssignTaxi assigns a taxi to a driver.
// Returns ErrAlreadyAssigned if the driver already has an active taxi assignment.
func (s *service) AssignTaxi(ctx context.Context, driverID, taxiID, ownerID uuid.UUID) error {
	// Verify driver belongs to owner.
	if _, err := s.repo.GetByID(ctx, driverID, ownerID); err != nil {
		return err
	}

	existing, err := s.repo.GetActiveAssignment(ctx, driverID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if existing != nil {
		return ErrAlreadyAssigned
	}

	_, err = s.repo.CreateAssignment(ctx, driverID, taxiID)
	return err
}

// UnassignTaxi unassigns the current active taxi from a driver.
func (s *service) UnassignTaxi(ctx context.Context, driverID, ownerID uuid.UUID) error {
	if _, err := s.repo.GetByID(ctx, driverID, ownerID); err != nil {
		return err
	}
	return s.repo.UnassignDriver(ctx, driverID)
}
