// Package receipt contains the Receipt domain entity, OCRClient interface, Processor, and Repository interface.
package receipt

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Sentinel errors.
var (
	ErrNotFound         = errors.New("receipt not found")
	ErrEmptyStorageURL  = errors.New("storage_url must not be empty before creating a receipt")
	ErrOCRUnavailable   = errors.New("OCR provider unavailable (built without CGO)")
)

// OCRStatus represents the lifecycle state of the OCR pipeline for a receipt.
type OCRStatus string

const (
	OCRStatusPending    OCRStatus = "pending"
	OCRStatusProcessing OCRStatus = "processing"
	OCRStatusDone       OCRStatus = "done"
	OCRStatusFailed     OCRStatus = "failed"
	OCRStatusSkipped    OCRStatus = "skipped"
)

// Receipt represents a receipt photo submitted by a driver.
type Receipt struct {
	ID               uuid.UUID
	DriverID         uuid.UUID
	TaxiID           uuid.UUID
	StorageURL       string           // NOT NULL — required before any DB write
	TelegramFileID   string
	OCRStatus        OCRStatus
	OCRRaw           []byte           // JSONB
	ExtractedTotal   *decimal.Decimal // NUMERIC(12,2) in COP
	ExtractedDate    *time.Time
	ExtractedVendor  *string
	ExtractedNIT     *string
	ExtractedCUFE    *string
	ExtractedConcept *string
	CreatedAt        time.Time
}

// OCRResult holds the fields extracted from a receipt image by the OCR pipeline.
type OCRResult struct {
	Total   *string // raw string from OCR, parsed later
	Date    *string
	Vendor  *string
	NIT     *string
	CUFE    *string
	Concept *string
	RawJSON []byte
}

// OCRClient is the interface for OCR providers. Implementations must be injected —
// they MUST NOT be instantiated directly by the Processor.
type OCRClient interface {
	ExtractData(ctx context.Context, imageBytes []byte) (*OCRResult, error)
}

// StorageClient is the interface for persistent object storage.
type StorageClient interface {
	Upload(ctx context.Context, key string, data []byte, contentType string) (url string, err error)
	Download(ctx context.Context, url string) ([]byte, error)
}

// Repository defines the persistence contract for receipts.
type Repository interface {
	Create(ctx context.Context, r *Receipt) (*Receipt, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Receipt, error)
	// ListPendingOCR selects up to 5 pending receipts ordered by created_at asc,
	// using FOR UPDATE SKIP LOCKED to prevent duplicate processing.
	ListPendingOCR(ctx context.Context) ([]*Receipt, error)
	UpdateOCRFields(ctx context.Context, id uuid.UUID, result *OCRResult) error
	SetOCRStatus(ctx context.Context, id uuid.UUID, status OCRStatus, rawJSON []byte) error
}

// Processor orchestrates the OCR pipeline for a single receipt.
type Processor interface {
	Process(ctx context.Context, receiptID uuid.UUID) error
}

// ExpenseAmountUpdater updates the amount of an expense linked to a receipt.
// Lives in receipt package to avoid import cycle with expense package.
type ExpenseAmountUpdater interface {
	UpdateAmountByReceiptID(ctx context.Context, receiptID uuid.UUID, amount decimal.Decimal) error
}
