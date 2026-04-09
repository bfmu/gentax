package worker_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/bmunoz/gentax/internal/receipt"
	"github.com/bmunoz/gentax/internal/worker"
)

// --- mock Processor ---

type mockProcessor struct {
	mock.Mock
}

func (m *mockProcessor) Process(ctx context.Context, receiptID uuid.UUID) error {
	args := m.Called(ctx, receiptID)
	return args.Error(0)
}

// TestOCRWorker_ProcessesPendingReceipts verifies that the worker calls Process for each
// pending receipt returned by ListPendingOCR.
func TestOCRWorker_ProcessesPendingReceipts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	id1 := uuid.New()
	id2 := uuid.New()

	r1 := &receipt.Receipt{ID: id1, StorageURL: "http://example.com/img1.png", OCRStatus: receipt.OCRStatusPending}
	r2 := &receipt.Receipt{ID: id2, StorageURL: "http://example.com/img2.png", OCRStatus: receipt.OCRStatusPending}

	mockRepo := &receipt.MockRepository{}
	mockProc := &mockProcessor{}

	// First poll: return 2 receipts; subsequent polls: return empty.
	var callCount int64
	mockRepo.On("ListPendingOCR", mock.Anything).Return([]*receipt.Receipt{r1, r2}, nil).Once()
	mockRepo.On("ListPendingOCR", mock.Anything).Run(func(args mock.Arguments) {
		// After the first batch is done, cancel context to stop the worker.
		if atomic.AddInt64(&callCount, 1) >= 1 {
			cancel()
		}
	}).Return([]*receipt.Receipt{}, nil)

	mockProc.On("Process", mock.Anything, id1).Return(nil)
	mockProc.On("Process", mock.Anything, id2).Return(nil)

	w := worker.NewOCRWorker(mockRepo, mockProc, 1, 50*time.Millisecond)
	w.Start(ctx)

	mockProc.AssertCalled(t, "Process", mock.Anything, id1)
	mockProc.AssertCalled(t, "Process", mock.Anything, id2)
}

// TestOCRWorker_StopsOnContextCancel verifies that cancelling the context causes
// the worker goroutines to exit cleanly.
func TestOCRWorker_StopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	mockRepo := &receipt.MockRepository{}
	mockProc := &mockProcessor{}

	// ListPendingOCR always returns empty so the worker just waits for the poll interval.
	mockRepo.On("ListPendingOCR", mock.Anything).Return([]*receipt.Receipt{}, nil)

	w := worker.NewOCRWorker(mockRepo, mockProc, 2, 50*time.Millisecond)

	done := make(chan struct{})
	go func() {
		w.Start(ctx)
		close(done)
	}()

	// Cancel the context and assert the worker stops promptly.
	cancel()
	select {
	case <-done:
		// good — worker exited
	case <-time.After(2 * time.Second):
		t.Fatal("OCRWorker did not stop within 2 seconds after context cancellation")
	}
}

// TestOCRWorker_ContinuesOnProcessError verifies that a process error for one receipt
// does not prevent the next receipt from being processed.
func TestOCRWorker_ContinuesOnProcessError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	id1 := uuid.New()
	id2 := uuid.New()

	r1 := &receipt.Receipt{ID: id1, StorageURL: "http://example.com/img1.png", OCRStatus: receipt.OCRStatusPending}
	r2 := &receipt.Receipt{ID: id2, StorageURL: "http://example.com/img2.png", OCRStatus: receipt.OCRStatusPending}

	mockRepo := &receipt.MockRepository{}
	mockProc := &mockProcessor{}

	var callCount int64
	mockRepo.On("ListPendingOCR", mock.Anything).Return([]*receipt.Receipt{r1, r2}, nil).Once()
	mockRepo.On("ListPendingOCR", mock.Anything).Run(func(_ mock.Arguments) {
		if atomic.AddInt64(&callCount, 1) >= 1 {
			cancel()
		}
	}).Return([]*receipt.Receipt{}, nil)

	// r1 fails, r2 succeeds.
	mockProc.On("Process", mock.Anything, id1).Return(errors.New("ocr provider timeout"))
	mockProc.On("Process", mock.Anything, id2).Return(nil)

	w := worker.NewOCRWorker(mockRepo, mockProc, 1, 50*time.Millisecond)
	w.Start(ctx)

	// Both receipts must have been attempted.
	mockProc.AssertCalled(t, "Process", mock.Anything, id1)
	mockProc.AssertCalled(t, "Process", mock.Anything, id2)

	// r2 processed successfully despite r1 failure.
	require.True(t, mockProc.AssertNumberOfCalls(t, "Process", 2))
	_ = assert.True // imported for use in OCR test
}
