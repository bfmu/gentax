package taxi

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const (
	minYear = 1990
)

type service struct {
	repo Repository
}

// NewService constructs a taxi Service backed by the given Repository.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// Create validates the input and persists a new taxi.
// Year validation is performed before calling the repository.
// owner_id is taken exclusively from input.OwnerID — never from request parameters.
func (s *service) Create(ctx context.Context, input CreateInput) (*Taxi, error) {
	currentYear := time.Now().Year()
	if input.Year < minYear || input.Year > currentYear+1 {
		return nil, ErrInvalidYear
	}

	tx, err := s.repo.Create(ctx, input)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// List returns all taxis belonging to the given owner.
// Filtering is delegated entirely to the repository (DB layer).
func (s *service) List(ctx context.Context, ownerID uuid.UUID) ([]*Taxi, error) {
	return s.repo.List(ctx, ownerID)
}

// Deactivate soft-deletes a taxi by setting active=false.
// It first verifies ownership via GetByID; if the taxi does not belong to ownerID
// it returns ErrNotFound (not a 403) to prevent resource enumeration.
func (s *service) Deactivate(ctx context.Context, id, ownerID uuid.UUID) error {
	_, err := s.repo.GetByID(ctx, id, ownerID)
	if err != nil {
		return err // propagates ErrNotFound
	}
	return s.repo.SetActive(ctx, id, ownerID, false)
}
