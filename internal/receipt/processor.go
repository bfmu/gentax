package receipt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// NotifyFunc is a callback invoked after OCR completes (success or failure).
// It should send a Telegram notification to the driver. Errors are logged but do not
// propagate — a notification failure MUST NOT cause Process to return an error.
// driverID is the UUID of the driver who submitted the receipt.
// result is nil when OCR failed — the callback should prompt manual amount entry.
type NotifyFunc func(ctx context.Context, driverID uuid.UUID, receiptID uuid.UUID, result *OCRResult) error

// processor implements Processor.
type processor struct {
	mu             sync.Mutex
	repo           Repository
	ocr            OCRClient
	storage        StorageClient
	notify         NotifyFunc
	expenseUpdater ExpenseAmountUpdater // optional; nil means no amount update
}

// ProcessorOption is a functional option for configuring a Processor.
type ProcessorOption func(*processor)

// WithExpenseAmountUpdater injects an optional expense amount updater.
// If set, after a successful OCR with a non-nil Total, the updater will be called.
func WithExpenseAmountUpdater(u ExpenseAmountUpdater) ProcessorOption {
	return func(p *processor) {
		p.expenseUpdater = u
	}
}

// NewProcessor creates a new Processor with the provided dependencies.
// notify may be nil; if nil, notifications are silently skipped.
// opts may include WithExpenseAmountUpdater.
func NewProcessor(repo Repository, ocr OCRClient, storage StorageClient, notify NotifyFunc, opts ...ProcessorOption) Processor {
	p := &processor{
		repo:    repo,
		ocr:     ocr,
		storage: storage,
		notify:  notify,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Process runs the OCR pipeline for a single receipt identified by receiptID.
//
//  1. GetByID — fetch receipt metadata
//  2. SetOCRStatus → "processing"
//  3. Download image bytes from storage_url
//  4. Call OCRClient.ExtractData
//  5. On success: UpdateOCRFields, SetOCRStatus → "done"
//  6. On failure: SetOCRStatus → "failed" with error JSON, do NOT return error
//  7. Call notify — log error if notify fails, do NOT return error
func (p *processor) Process(ctx context.Context, receiptID uuid.UUID) error {
	r, err := p.repo.GetByID(ctx, receiptID)
	if err != nil {
		return fmt.Errorf("processor: get receipt %s: %w", receiptID, err)
	}

	if err := p.repo.SetOCRStatus(ctx, receiptID, OCRStatusProcessing, nil); err != nil {
		return fmt.Errorf("processor: set status processing: %w", err)
	}

	imageBytes, err := p.downloadImage(ctx, r.StorageURL)
	if err != nil {
		return p.failReceipt(ctx, r, fmt.Errorf("download image: %w", err))
	}

	ocrResult, err := p.ocr.ExtractData(ctx, imageBytes)
	if err != nil {
		return p.failReceipt(ctx, r, fmt.Errorf("ocr extract: %w", err))
	}

	if err := p.repo.UpdateOCRFields(ctx, receiptID, ocrResult); err != nil {
		return p.failReceipt(ctx, r, fmt.Errorf("update ocr fields: %w", err))
	}

	if err := p.repo.SetOCRStatus(ctx, receiptID, OCRStatusDone, ocrResult.RawJSON); err != nil {
		// Best-effort: log but don't fail — data is already written.
		slog.Error("processor: set status done", "receipt_id", receiptID, "error", err)
	}

	// Update expense amount if OCR extracted a total and updater is wired.
	if p.expenseUpdater != nil && ocrResult.Total != nil {
		if amount, parseErr := decimal.NewFromString(*ocrResult.Total); parseErr == nil {
			if updErr := p.expenseUpdater.UpdateAmountByReceiptID(ctx, receiptID, amount); updErr != nil {
				slog.Warn("processor: failed to update expense amount", "receipt_id", receiptID, "error", updErr)
			}
		}
	}

	p.sendNotify(ctx, r, ocrResult)
	return nil
}

// failReceipt marks the receipt as failed, stores error JSON, logs, and returns nil
// so the worker can continue processing other receipts.
// It also notifies the driver (with result=nil) so they can enter the amount manually.
func (p *processor) failReceipt(ctx context.Context, r *Receipt, cause error) error {
	slog.Error("processor: ocr failed", "receipt_id", r.ID, "error", cause)

	errJSON, _ := json.Marshal(map[string]string{"error": cause.Error()})
	if setErr := p.repo.SetOCRStatus(ctx, r.ID, OCRStatusFailed, errJSON); setErr != nil {
		slog.Error("processor: set status failed", "receipt_id", r.ID, "error", setErr)
	}
	p.sendNotify(ctx, r, nil) // nil result = OCR failed, prompt manual entry
	return nil                // do NOT propagate — the worker should continue
}

// downloadImage fetches image bytes from a URL, trying StorageClient first then HTTP GET.
func (p *processor) downloadImage(ctx context.Context, storageURL string) ([]byte, error) {
	if p.storage != nil {
		data, err := p.storage.Download(ctx, storageURL)
		if err == nil {
			return data, nil
		}
		slog.Warn("processor: storage download failed, falling back to HTTP", "url", storageURL, "error", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, storageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build http request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http get returned %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// sendNotify calls the notify function if set, logging errors without propagating them.
func (p *processor) sendNotify(ctx context.Context, r *Receipt, result *OCRResult) {
	p.mu.Lock()
	fn := p.notify
	p.mu.Unlock()
	if fn == nil {
		return
	}
	if err := fn(ctx, r.DriverID, r.ID, result); err != nil {
		slog.Error("processor: notify failed", "receipt_id", r.ID, "error", err)
	}
}

// SetNotify replaces the OCR result notification callback. Thread-safe; may be called
// after construction (e.g., after the Telegram bot is initialised).
func (p *processor) SetNotify(fn NotifyFunc) {
	p.mu.Lock()
	p.notify = fn
	p.mu.Unlock()
}
