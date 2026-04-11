package driver

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockRepository is a testify/mock implementation of Repository.
type MockRepository struct {
	mock.Mock
}

// Create mocks the Create method.
func (m *MockRepository) Create(ctx context.Context, input CreateInput) (*Driver, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Driver), args.Error(1)
}

// GetByID mocks the GetByID method.
func (m *MockRepository) GetByID(ctx context.Context, id, ownerID uuid.UUID) (*Driver, error) {
	args := m.Called(ctx, id, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Driver), args.Error(1)
}

// GetByTelegramID mocks the GetByTelegramID method.
func (m *MockRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*Driver, error) {
	args := m.Called(ctx, telegramID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Driver), args.Error(1)
}

// GetByLinkToken mocks the GetByLinkToken method.
func (m *MockRepository) GetByLinkToken(ctx context.Context, token string) (*Driver, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Driver), args.Error(1)
}

// List mocks the List method.
func (m *MockRepository) List(ctx context.Context, ownerID uuid.UUID) ([]*Driver, error) {
	args := m.Called(ctx, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Driver), args.Error(1)
}

// SetActive mocks the SetActive method.
func (m *MockRepository) SetActive(ctx context.Context, id, ownerID uuid.UUID, active bool) error {
	args := m.Called(ctx, id, ownerID, active)
	return args.Error(0)
}

// SetLinkToken mocks the SetLinkToken method.
func (m *MockRepository) SetLinkToken(ctx context.Context, driverID uuid.UUID, token string, expiresAt time.Time) error {
	args := m.Called(ctx, driverID, token, expiresAt)
	return args.Error(0)
}

// MarkLinkTokenUsed mocks the MarkLinkTokenUsed method.
func (m *MockRepository) MarkLinkTokenUsed(ctx context.Context, driverID uuid.UUID) error {
	args := m.Called(ctx, driverID)
	return args.Error(0)
}

// SetTelegramID mocks the SetTelegramID method.
func (m *MockRepository) SetTelegramID(ctx context.Context, driverID uuid.UUID, telegramID int64) error {
	args := m.Called(ctx, driverID, telegramID)
	return args.Error(0)
}

// GetActiveAssignment mocks the GetActiveAssignment method.
func (m *MockRepository) GetActiveAssignment(ctx context.Context, driverID uuid.UUID) (*Assignment, error) {
	args := m.Called(ctx, driverID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Assignment), args.Error(1)
}

// CreateAssignment mocks the CreateAssignment method.
func (m *MockRepository) CreateAssignment(ctx context.Context, driverID, taxiID uuid.UUID) (*Assignment, error) {
	args := m.Called(ctx, driverID, taxiID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Assignment), args.Error(1)
}

// UnassignDriver mocks the UnassignDriver method.
func (m *MockRepository) UnassignDriver(ctx context.Context, driverID uuid.UUID) error {
	args := m.Called(ctx, driverID)
	return args.Error(0)
}

// ListWithAssignment mocks the ListWithAssignment method.
func (m *MockRepository) ListWithAssignment(ctx context.Context, ownerID uuid.UUID) ([]*DriverWithAssignment, error) {
	args := m.Called(ctx, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*DriverWithAssignment), args.Error(1)
}
