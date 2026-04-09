package receipt

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockStorageClient is a testify mock for StorageClient.
type MockStorageClient struct {
	mock.Mock
}

// Upload mocks StorageClient.Upload.
func (m *MockStorageClient) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	args := m.Called(ctx, key, data, contentType)
	return args.String(0), args.Error(1)
}

// Download mocks StorageClient.Download.
func (m *MockStorageClient) Download(ctx context.Context, url string) ([]byte, error) {
	args := m.Called(ctx, url)
	res, _ := args.Get(0).([]byte)
	return res, args.Error(1)
}
