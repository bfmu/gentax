package receipt

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
)

// MockExpenseAmountUpdater is a testify mock for ExpenseAmountUpdater.
type MockExpenseAmountUpdater struct {
	mock.Mock
}

// UpdateAmountByReceiptID mocks ExpenseAmountUpdater.UpdateAmountByReceiptID.
func (m *MockExpenseAmountUpdater) UpdateAmountByReceiptID(ctx context.Context, receiptID uuid.UUID, amount decimal.Decimal) error {
	args := m.Called(ctx, receiptID, amount)
	return args.Error(0)
}
