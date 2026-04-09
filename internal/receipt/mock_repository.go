package receipt

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockRepository is a testify mock for Repository.
type MockRepository struct {
	mock.Mock
}

// Create mocks Repository.Create.
func (m *MockRepository) Create(ctx context.Context, r *Receipt) (*Receipt, error) {
	args := m.Called(ctx, r)
	res, _ := args.Get(0).(*Receipt)
	return res, args.Error(1)
}

// GetByID mocks Repository.GetByID.
func (m *MockRepository) GetByID(ctx context.Context, id uuid.UUID) (*Receipt, error) {
	args := m.Called(ctx, id)
	res, _ := args.Get(0).(*Receipt)
	return res, args.Error(1)
}

// ListPendingOCR mocks Repository.ListPendingOCR.
func (m *MockRepository) ListPendingOCR(ctx context.Context) ([]*Receipt, error) {
	args := m.Called(ctx)
	res, _ := args.Get(0).([]*Receipt)
	return res, args.Error(1)
}

// UpdateOCRFields mocks Repository.UpdateOCRFields.
func (m *MockRepository) UpdateOCRFields(ctx context.Context, id uuid.UUID, result *OCRResult) error {
	args := m.Called(ctx, id, result)
	return args.Error(0)
}

// SetOCRStatus mocks Repository.SetOCRStatus.
func (m *MockRepository) SetOCRStatus(ctx context.Context, id uuid.UUID, status OCRStatus, rawJSON []byte) error {
	args := m.Called(ctx, id, status, rawJSON)
	return args.Error(0)
}
