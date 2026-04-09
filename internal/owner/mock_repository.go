package owner

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockRepository is a testify/mock implementation of Repository.
type MockRepository struct {
	mock.Mock
}

// Create mocks the Create method.
func (m *MockRepository) Create(ctx context.Context, name, email, passwordHash string) (*Owner, error) {
	args := m.Called(ctx, name, email, passwordHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Owner), args.Error(1)
}

// GetByEmail mocks the GetByEmail method.
func (m *MockRepository) GetByEmail(ctx context.Context, email string) (*Owner, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Owner), args.Error(1)
}

// Count mocks the Count method.
func (m *MockRepository) Count(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}
