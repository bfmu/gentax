package expense

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
)

// MockRepository is a testify/mock implementation of Repository for unit tests.
type MockRepository struct {
	mock.Mock
}

// Create mocks the Create method.
func (m *MockRepository) Create(ctx context.Context, input CreateInput) (*Expense, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Expense), args.Error(1)
}

// GetByID mocks the GetByID method.
func (m *MockRepository) GetByID(ctx context.Context, id, ownerID uuid.UUID) (*Expense, error) {
	args := m.Called(ctx, id, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Expense), args.Error(1)
}

// List mocks the List method.
func (m *MockRepository) List(ctx context.Context, filter ListFilter) ([]*Expense, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Expense), args.Error(1)
}

// UpdateStatus mocks the UpdateStatus method.
func (m *MockRepository) UpdateStatus(ctx context.Context, id, ownerID uuid.UUID, status Status, reviewedBy *uuid.UUID, rejectionReason string) error {
	args := m.Called(ctx, id, ownerID, status, reviewedBy, rejectionReason)
	return args.Error(0)
}

// UpdateAmount mocks the UpdateAmount method.
func (m *MockRepository) UpdateAmount(ctx context.Context, id uuid.UUID, amount decimal.Decimal) error {
	args := m.Called(ctx, id, amount)
	return args.Error(0)
}

// SumByTaxi mocks the SumByTaxi method.
func (m *MockRepository) SumByTaxi(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*TaxiSummary, error) {
	args := m.Called(ctx, ownerID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*TaxiSummary), args.Error(1)
}

// SumByDriver mocks the SumByDriver method.
func (m *MockRepository) SumByDriver(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*DriverSummary, error) {
	args := m.Called(ctx, ownerID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*DriverSummary), args.Error(1)
}

// SumByCategory mocks the SumByCategory method.
func (m *MockRepository) SumByCategory(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*CategorySummary, error) {
	args := m.Called(ctx, ownerID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*CategorySummary), args.Error(1)
}
