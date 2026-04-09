package receipt

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockOCRClient is a testify mock for OCRClient.
type MockOCRClient struct {
	mock.Mock
}

// ExtractData mocks OCRClient.ExtractData.
func (m *MockOCRClient) ExtractData(ctx context.Context, imageBytes []byte) (*OCRResult, error) {
	args := m.Called(ctx, imageBytes)
	res, _ := args.Get(0).(*OCRResult)
	return res, args.Error(1)
}
