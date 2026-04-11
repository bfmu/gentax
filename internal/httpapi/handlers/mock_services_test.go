package handlers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"

	"github.com/bmunoz/gentax/internal/auth"
	"github.com/bmunoz/gentax/internal/driver"
	"github.com/bmunoz/gentax/internal/expense"
	"github.com/bmunoz/gentax/internal/taxi"
)

// ----- mock taxi.Service -----

type mockTaxiService struct{ mock.Mock }

func (m *mockTaxiService) Create(ctx context.Context, input taxi.CreateInput) (*taxi.Taxi, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*taxi.Taxi), args.Error(1)
}

func (m *mockTaxiService) List(ctx context.Context, ownerID uuid.UUID) ([]*taxi.Taxi, error) {
	args := m.Called(ctx, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*taxi.Taxi), args.Error(1)
}

func (m *mockTaxiService) Deactivate(ctx context.Context, id, ownerID uuid.UUID) error {
	args := m.Called(ctx, id, ownerID)
	return args.Error(0)
}

// ----- mock driver.Service -----

type mockDriverService struct{ mock.Mock }

func (m *mockDriverService) Create(ctx context.Context, input driver.CreateInput) (*driver.Driver, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*driver.Driver), args.Error(1)
}

func (m *mockDriverService) GenerateLinkToken(ctx context.Context, driverID, ownerID uuid.UUID) (string, error) {
	args := m.Called(ctx, driverID, ownerID)
	return args.String(0), args.Error(1)
}

func (m *mockDriverService) LinkTelegramID(ctx context.Context, token string, telegramID int64) error {
	args := m.Called(ctx, token, telegramID)
	return args.Error(0)
}

func (m *mockDriverService) Deactivate(ctx context.Context, id, ownerID uuid.UUID) error {
	args := m.Called(ctx, id, ownerID)
	return args.Error(0)
}

func (m *mockDriverService) List(ctx context.Context, ownerID uuid.UUID) ([]*driver.Driver, error) {
	args := m.Called(ctx, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*driver.Driver), args.Error(1)
}

func (m *mockDriverService) ListWithAssignment(ctx context.Context, ownerID uuid.UUID) ([]*driver.DriverWithAssignment, error) {
	args := m.Called(ctx, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*driver.DriverWithAssignment), args.Error(1)
}

func (m *mockDriverService) AssignTaxi(ctx context.Context, driverID, taxiID, ownerID uuid.UUID) error {
	args := m.Called(ctx, driverID, taxiID, ownerID)
	return args.Error(0)
}

func (m *mockDriverService) UnassignTaxi(ctx context.Context, driverID, ownerID uuid.UUID) error {
	args := m.Called(ctx, driverID, ownerID)
	return args.Error(0)
}

// ----- mock expense.Service -----

type mockExpenseService struct{ mock.Mock }

func (m *mockExpenseService) Create(ctx context.Context, input expense.CreateInput) (*expense.Expense, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*expense.Expense), args.Error(1)
}

func (m *mockExpenseService) Confirm(ctx context.Context, id, driverID uuid.UUID) error {
	args := m.Called(ctx, id, driverID)
	return args.Error(0)
}

func (m *mockExpenseService) Approve(ctx context.Context, id, ownerID uuid.UUID) error {
	args := m.Called(ctx, id, ownerID)
	return args.Error(0)
}

func (m *mockExpenseService) Reject(ctx context.Context, id, ownerID uuid.UUID, reason string) error {
	args := m.Called(ctx, id, ownerID, reason)
	return args.Error(0)
}

func (m *mockExpenseService) List(ctx context.Context, filter expense.ListFilter) ([]*expense.Expense, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*expense.Expense), args.Error(1)
}

func (m *mockExpenseService) GetByID(ctx context.Context, id, ownerID uuid.UUID) (*expense.Expense, error) {
	args := m.Called(ctx, id, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*expense.Expense), args.Error(1)
}

func (m *mockExpenseService) UpdateAmount(ctx context.Context, id uuid.UUID, amount decimal.Decimal) error {
	args := m.Called(ctx, id, amount)
	return args.Error(0)
}

func (m *mockExpenseService) SumByTaxi(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*expense.TaxiSummary, error) {
	args := m.Called(ctx, ownerID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*expense.TaxiSummary), args.Error(1)
}

func (m *mockExpenseService) SumByDriver(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*expense.DriverSummary, error) {
	args := m.Called(ctx, ownerID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*expense.DriverSummary), args.Error(1)
}

func (m *mockExpenseService) SumByCategory(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]*expense.CategorySummary, error) {
	args := m.Called(ctx, ownerID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*expense.CategorySummary), args.Error(1)
}

// ----- mock DriverFinder -----

type mockDriverFinder struct{ mock.Mock }

func (m *mockDriverFinder) GetByTelegramID(ctx context.Context, telegramID int64) (*driver.Driver, error) {
	args := m.Called(ctx, telegramID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*driver.Driver), args.Error(1)
}

// ----- mock auth.TokenIssuer -----

type mockTokenIssuer struct{ mock.Mock }

func (m *mockTokenIssuer) Issue(claims auth.Claims, ttl time.Duration) (string, error) {
	args := m.Called(claims, ttl)
	return args.String(0), args.Error(1)
}
