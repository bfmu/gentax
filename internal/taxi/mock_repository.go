package taxi

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockRepository is a testify/mock implementation of Repository.
type MockRepository struct {
	mock.Mock
}

// Create mocks the Create method.
func (m *MockRepository) Create(ctx context.Context, input CreateInput) (*Taxi, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Taxi), args.Error(1)
}

// GetByID mocks the GetByID method.
func (m *MockRepository) GetByID(ctx context.Context, id, ownerID uuid.UUID) (*Taxi, error) {
	args := m.Called(ctx, id, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Taxi), args.Error(1)
}

// List mocks the List method.
func (m *MockRepository) List(ctx context.Context, ownerID uuid.UUID) ([]*Taxi, error) {
	args := m.Called(ctx, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Taxi), args.Error(1)
}

// SetActive mocks the SetActive method.
func (m *MockRepository) SetActive(ctx context.Context, id, ownerID uuid.UUID, active bool) error {
	args := m.Called(ctx, id, ownerID, active)
	return args.Error(0)
}
