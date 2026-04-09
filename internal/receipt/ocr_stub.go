//go:build nocgo

package receipt

import "context"

// TesseractClient is a no-op stub when CGO is disabled (build tag: nocgo).
type TesseractClient struct{}

// NewTesseractClient returns a stub TesseractClient that always returns ErrOCRUnavailable.
func NewTesseractClient() *TesseractClient {
	return &TesseractClient{}
}

// ExtractData always returns ErrOCRUnavailable when the nocgo build tag is set.
func (c *TesseractClient) ExtractData(_ context.Context, _ []byte) (*OCRResult, error) {
	return nil, ErrOCRUnavailable
}
