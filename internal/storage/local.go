// Package storage provides storage client implementations.
package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// LocalStorageClient stores files on the local filesystem under a configurable base directory.
// It implements receipt.StorageClient and is suitable for VPS deployments without S3/GCS.
type LocalStorageClient struct {
	baseDir string
}

// NewLocalStorageClient creates a LocalStorageClient that stores files under baseDir.
// The directory is created (with parents) if it does not exist.
func NewLocalStorageClient(baseDir string) (*LocalStorageClient, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("storage: create base dir %q: %w", baseDir, err)
	}
	return &LocalStorageClient{baseDir: baseDir}, nil
}

// Upload writes data to baseDir/key and returns a file:// URL.
// The key is treated as a relative filename (e.g. "receipts/uuid.jpg").
// Parent directories inside baseDir are created automatically.
func (c *LocalStorageClient) Upload(_ context.Context, key string, data []byte, _ string) (string, error) {
	dest := filepath.Join(c.baseDir, key)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("storage: create dir for %q: %w", dest, err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return "", fmt.Errorf("storage: write %q: %w", dest, err)
	}
	return "file://" + dest, nil
}

// Download reads and returns the bytes stored at url.
// url must be a file:// URL produced by Upload.
func (c *LocalStorageClient) Download(_ context.Context, url string) ([]byte, error) {
	const prefix = "file://"
	if len(url) <= len(prefix) || url[:len(prefix)] != prefix {
		return nil, fmt.Errorf("storage: unsupported URL scheme %q", url)
	}
	path := url[len(prefix):]
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("storage: read %q: %w", path, err)
	}
	return data, nil
}
