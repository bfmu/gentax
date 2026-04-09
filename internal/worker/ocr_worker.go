// Package worker contains the OCR goroutine pool that polls pending receipts and calls the OCR provider.
package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bmunoz/gentax/internal/receipt"
)

// OCRWorker polls the database for pending receipts and processes them via an OCR pipeline.
// It runs a configurable number of goroutines concurrently.
type OCRWorker struct {
	repo         receipt.Repository
	processor    receipt.Processor
	poolSize     int
	pollInterval time.Duration
	maxRetries   int
}

// NewOCRWorker creates a new OCRWorker.
// poolSize controls how many goroutines poll in parallel.
// pollInterval is the delay between polls when no pending receipts are found.
func NewOCRWorker(
	repo receipt.Repository,
	processor receipt.Processor,
	poolSize int,
	pollInterval time.Duration,
) *OCRWorker {
	if poolSize <= 0 {
		poolSize = 1
	}
	if pollInterval <= 0 {
		pollInterval = 10 * time.Second
	}
	return &OCRWorker{
		repo:         repo,
		processor:    processor,
		poolSize:     poolSize,
		pollInterval: pollInterval,
		maxRetries:   3,
	}
}

// Start launches poolSize goroutines, each running an independent poll loop.
// The loops run until ctx is cancelled. Start blocks until all goroutines exit.
func (w *OCRWorker) Start(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < w.poolSize; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			w.runLoop(ctx, workerID)
		}(i)
	}
	wg.Wait()
}

// runLoop is the main poll loop for a single worker goroutine.
// Panics inside the loop are recovered and logged — the worker does NOT crash.
func (w *OCRWorker) runLoop(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			slog.Info("ocr_worker: shutting down", "worker_id", workerID)
			return
		default:
		}

		w.safeProcessBatch(ctx, workerID)

		// Wait for the next poll interval or context cancellation.
		select {
		case <-ctx.Done():
			slog.Info("ocr_worker: shutting down", "worker_id", workerID)
			return
		case <-time.After(w.pollInterval):
		}
	}
}

// safeProcessBatch wraps processBatch with a recover so a panic does not crash the worker.
func (w *OCRWorker) safeProcessBatch(ctx context.Context, workerID int) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("ocr_worker: recovered from panic", "worker_id", workerID, "panic", r)
		}
	}()
	w.processBatch(ctx, workerID)
}

// processBatch polls the repository for pending receipts and processes each one.
// A failure on one receipt is logged but does NOT stop processing of the next.
func (w *OCRWorker) processBatch(ctx context.Context, workerID int) {
	pending, err := w.repo.ListPendingOCR(ctx)
	if err != nil {
		slog.Error("ocr_worker: list pending receipts failed", "worker_id", workerID, "error", err)
		return
	}

	for _, r := range pending {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := w.processor.Process(ctx, r.ID); err != nil {
			slog.Error("ocr_worker: process receipt failed",
				"worker_id", workerID,
				"receipt_id", r.ID,
				"error", err,
			)
			// Continue — process next receipt regardless of this failure.
		}
	}
}
