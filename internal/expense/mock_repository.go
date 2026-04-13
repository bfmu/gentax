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

// GetByReceiptID mocks the GetByReceiptID method.
func (m *MockRepository) GetByReceiptID(ctx context.Context, receiptID uuid.UUID) (*Expense, error) {
	args := m.Called(ctx, receiptID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Expense), args.Error(1)
}

// GetReceiptStorageURL mocks the GetReceiptStorageURL method.
func (m *MockRepository) GetReceiptStorageURL(ctx context.Context, id, ownerID uuid.UUID) (string, error) {
	args := m.Called(ctx, id, ownerID)
	return args.String(0), args.Error(1)
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

// UpdateReceiptID mocks the UpdateReceiptID method.
func (m *MockRepository) UpdateReceiptID(ctx context.Context, id uuid.UUID, receiptID uuid.UUID) error {
	args := m.Called(ctx, id, receiptID)
	return args.Error(0)
}

// ListCategories mocks the ListCategories method.
func (m *MockRepository) ListCategories(ctx context.Context, ownerID uuid.UUID) ([]*ExpenseCategory, error) {
	args := m.Called(ctx, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ExpenseCategory), args.Error(1)
}

// CreateCategory mocks the CreateCategory method.
func (m *MockRepository) CreateCategory(ctx context.Context, ownerID uuid.UUID, name string) (*ExpenseCategory, error) {
	args := m.Called(ctx, ownerID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ExpenseCategory), args.Error(1)
}

// DeleteCategory mocks the DeleteCategory method.
func (m *MockRepository) DeleteCategory(ctx context.Context, id, ownerID uuid.UUID) error {
	return m.Called(ctx, id, ownerID).Error(0)
}

// SeedDefaultCategories mocks the SeedDefaultCategories method.
func (m *MockRepository) SeedDefaultCategories(ctx context.Context, ownerID uuid.UUID) error {
	return m.Called(ctx, ownerID).Error(0)
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

// AddAttachment mocks the AddAttachment method.
func (m *MockRepository) AddAttachment(ctx context.Context, expenseID, receiptID uuid.UUID, label string) (*Attachment, error) {
	args := m.Called(ctx, expenseID, receiptID, label)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Attachment), args.Error(1)
}

// ListAttachments mocks the ListAttachments method.
func (m *MockRepository) ListAttachments(ctx context.Context, expenseID uuid.UUID) ([]Attachment, error) {
	args := m.Called(ctx, expenseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Attachment), args.Error(1)
}
