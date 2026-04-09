package owner

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type service struct {
	repo Repository
}

// NewService returns a Service backed by the given Repository.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// Authenticate verifies email and plainPassword against the stored bcrypt hash.
// Returns ErrInvalidCredentials for any auth failure (not-found or wrong password).
func (s *service) Authenticate(ctx context.Context, email, plainPassword string) (*Owner, error) {
	o, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("owner.Service.Authenticate: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(o.PasswordHash), []byte(plainPassword)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return o, nil
}

// Count returns the total number of owners.
func (s *service) Count(ctx context.Context) (int, error) {
	return s.repo.Count(ctx)
}

// Create hashes plainPassword with bcrypt and stores the new owner.
func (s *service) Create(ctx context.Context, name, email, plainPassword string) (*Owner, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("owner.Service.Create: hash password: %w", err)
	}

	o, err := s.repo.Create(ctx, name, email, string(hash))
	if err != nil {
		return nil, err
	}
	return o, nil
}
