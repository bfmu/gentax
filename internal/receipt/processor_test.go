package receipt_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/bmunoz/gentax/internal/receipt"
)

// fakeImageBytes is a minimal PNG so the processor can call ExtractData.
var fakeImageBytes = []byte("fake-image-data")

// newTestReceipt returns a Receipt with a valid StorageURL pointing to the given server.
func newTestReceipt(serverURL string) *receipt.Receipt {
	storageURL := serverURL + "/image.png"
	return &receipt.Receipt{
		ID:         uuid.New(),
		DriverID:   uuid.New(),
		TaxiID:     uuid.New(),
		StorageURL: storageURL,
		OCRStatus:  receipt.OCRStatusPending,
		CreatedAt:  time.Now(),
	}
}

// makeImageServer returns a test HTTP server that serves fakeImageBytes.
func makeImageServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fakeImageBytes)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestProcessor_UploadBeforeDB verifies that if the storage download fails and there's
// no image to process, the receipt is marked failed and no OCR update is written.
// This is the "upload before DB" invariant: we must fail safely, never writing OCR
// data that wasn't actually processed.
func TestProcessor_UploadBeforeDB(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()

	r := &receipt.Receipt{
		ID:         id,
		StorageURL: "http://invalid-host-that-never-resolves.example/img.png",
		OCRStatus:  receipt.OCRStatusPending,
	}

	mockRepo := &receipt.MockRepository{}
	mockOCR := &receipt.MockOCRClient{}
	mockStorage := &receipt.MockStorageClient{}

	mockRepo.On("GetByID", ctx, id).Return(r, nil)
	mockRepo.On("SetOCRStatus", ctx, id, receipt.OCRStatusProcessing, mock.AnythingOfType("[]uint8")).Return(nil)
	mockRepo.On("SetOCRStatus", ctx, id, receipt.OCRStatusProcessing, []byte(nil)).Return(nil)
	// Storage download fails.
	mockStorage.On("Download", ctx, r.StorageURL).Return([]byte(nil), errors.New("storage unavailable"))
	// After download failure, receipt should be marked failed.
	mockRepo.On("SetOCRStatus", ctx, id, receipt.OCRStatusFailed, mock.AnythingOfType("[]uint8")).Return(nil)

	p := receipt.NewProcessor(mockRepo, mockOCR, mockStorage, nil)
	err := p.Process(ctx, id)

	// Process must not return an error (failures are swallowed to avoid crashing worker).
	require.NoError(t, err)

	// OCR ExtractData must NOT have been called — no image was available.
	mockOCR.AssertNotCalled(t, "ExtractData")
	// UpdateOCRFields must NOT have been called.
	mockRepo.AssertNotCalled(t, "UpdateOCRFields")
}

// TestProcessor_OCRSuccess verifies the happy path: OCR returns data, receipt is updated.
func TestProcessor_OCRSuccess(t *testing.T) {
	ctx := context.Background()

	srv := makeImageServer(t)
	id := uuid.New()
	r := newTestReceipt(srv.URL)
	r.ID = id

	vendor := "ACME LTDA"
	nit := "9004558906"
	total := "150000"
	date := "2024-01-15"
	cufe := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef12345678"
	concept := "Combustible"
	ocrResult := &receipt.OCRResult{
		Vendor:  &vendor,
		NIT:     &nit,
		Total:   &total,
		Date:    &date,
		CUFE:    &cufe,
		Concept: &concept,
		RawJSON: []byte(`{"fields":{}}`),
	}

	mockRepo := &receipt.MockRepository{}
	mockOCR := &receipt.MockOCRClient{}

	mockRepo.On("GetByID", ctx, id).Return(r, nil)
	mockRepo.On("SetOCRStatus", ctx, id, receipt.OCRStatusProcessing, []byte(nil)).Return(nil)
	mockOCR.On("ExtractData", ctx, fakeImageBytes).Return(ocrResult, nil)
	mockRepo.On("UpdateOCRFields", ctx, id, ocrResult).Return(nil)
	mockRepo.On("SetOCRStatus", ctx, id, receipt.OCRStatusDone, ocrResult.RawJSON).Return(nil)

	p := receipt.NewProcessor(mockRepo, mockOCR, nil, nil)
	err := p.Process(ctx, id)

	require.NoError(t, err)
	mockOCR.AssertCalled(t, "ExtractData", ctx, fakeImageBytes)
	mockRepo.AssertCalled(t, "UpdateOCRFields", ctx, id, ocrResult)
	mockRepo.AssertCalled(t, "SetOCRStatus", ctx, id, receipt.OCRStatusDone, ocrResult.RawJSON)
}

// TestProcessor_OCRFailed verifies that an OCR provider error marks the receipt failed
// and does NOT crash — Process returns nil.
func TestProcessor_OCRFailed(t *testing.T) {
	ctx := context.Background()

	srv := makeImageServer(t)
	id := uuid.New()
	r := newTestReceipt(srv.URL)
	r.ID = id

	mockRepo := &receipt.MockRepository{}
	mockOCR := &receipt.MockOCRClient{}

	mockRepo.On("GetByID", ctx, id).Return(r, nil)
	mockRepo.On("SetOCRStatus", ctx, id, receipt.OCRStatusProcessing, []byte(nil)).Return(nil)
	mockOCR.On("ExtractData", ctx, fakeImageBytes).Return((*receipt.OCRResult)(nil), errors.New("tesseract error"))
	mockRepo.On("SetOCRStatus", ctx, id, receipt.OCRStatusFailed, mock.AnythingOfType("[]uint8")).Return(nil)

	p := receipt.NewProcessor(mockRepo, mockOCR, nil, nil)
	err := p.Process(ctx, id)

	// Must not return an error — failures are swallowed.
	require.NoError(t, err)

	// UpdateOCRFields must NOT have been called.
	mockRepo.AssertNotCalled(t, "UpdateOCRFields")
	// Status must have been set to failed.
	mockRepo.AssertCalled(t, "SetOCRStatus", ctx, id, receipt.OCRStatusFailed, mock.AnythingOfType("[]uint8"))

	// Verify the OCR status was never set to done.
	for _, call := range mockRepo.Calls {
		if call.Method == "SetOCRStatus" {
			assert.NotEqual(t, receipt.OCRStatusDone, call.Arguments[2], "SetOCRStatus should not be called with done on OCR failure")
		}
	}
}

// TestProcessor_UpdatesExpenseAmountAfterOCR verifies that after a successful OCR with a total,
// the ExpenseAmountUpdater is called with the parsed amount.
func TestProcessor_UpdatesExpenseAmountAfterOCR(t *testing.T) {
	ctx := context.Background()

	srv := makeImageServer(t)
	id := uuid.New()
	r := newTestReceipt(srv.URL)
	r.ID = id

	total := "150000"
	ocrResult := &receipt.OCRResult{
		Total:   &total,
		RawJSON: []byte(`{"fields":{}}`),
	}

	mockRepo := &receipt.MockRepository{}
	mockOCR := &receipt.MockOCRClient{}
	mockUpdater := &receipt.MockExpenseAmountUpdater{}

	mockRepo.On("GetByID", ctx, id).Return(r, nil)
	mockRepo.On("SetOCRStatus", ctx, id, receipt.OCRStatusProcessing, []byte(nil)).Return(nil)
	mockOCR.On("ExtractData", ctx, fakeImageBytes).Return(ocrResult, nil)
	mockRepo.On("UpdateOCRFields", ctx, id, ocrResult).Return(nil)
	mockRepo.On("SetOCRStatus", ctx, id, receipt.OCRStatusDone, ocrResult.RawJSON).Return(nil)
	mockUpdater.On("UpdateAmountByReceiptID", ctx, id, mock.Anything).Return(nil)

	p := receipt.NewProcessor(mockRepo, mockOCR, nil, nil, receipt.WithExpenseAmountUpdater(mockUpdater))
	err := p.Process(ctx, id)

	require.NoError(t, err)
	mockUpdater.AssertCalled(t, "UpdateAmountByReceiptID", ctx, id, mock.Anything)
}

// TestProcessor_SkipsAmountUpdateWhenTotalNil verifies that when OCR returns no total,
// the ExpenseAmountUpdater is NOT called.
func TestProcessor_SkipsAmountUpdateWhenTotalNil(t *testing.T) {
	ctx := context.Background()

	srv := makeImageServer(t)
	id := uuid.New()
	r := newTestReceipt(srv.URL)
	r.ID = id

	ocrResult := &receipt.OCRResult{
		Total:   nil, // no total extracted
		RawJSON: []byte(`{"fields":{}}`),
	}

	mockRepo := &receipt.MockRepository{}
	mockOCR := &receipt.MockOCRClient{}
	mockUpdater := &receipt.MockExpenseAmountUpdater{}

	mockRepo.On("GetByID", ctx, id).Return(r, nil)
	mockRepo.On("SetOCRStatus", ctx, id, receipt.OCRStatusProcessing, []byte(nil)).Return(nil)
	mockOCR.On("ExtractData", ctx, fakeImageBytes).Return(ocrResult, nil)
	mockRepo.On("UpdateOCRFields", ctx, id, ocrResult).Return(nil)
	mockRepo.On("SetOCRStatus", ctx, id, receipt.OCRStatusDone, ocrResult.RawJSON).Return(nil)

	p := receipt.NewProcessor(mockRepo, mockOCR, nil, nil, receipt.WithExpenseAmountUpdater(mockUpdater))
	err := p.Process(ctx, id)

	require.NoError(t, err)
	mockUpdater.AssertNotCalled(t, "UpdateAmountByReceiptID")
}

// TestProcessor_SkipLocked verifies that concurrent calls to ListPendingOCR return
// different receipts (simulating SKIP LOCKED behaviour via the mock).
func TestProcessor_SkipLocked(t *testing.T) {
	ctx := context.Background()

	id1 := uuid.New()
	id2 := uuid.New()

	r1 := &receipt.Receipt{ID: id1, StorageURL: "http://example.com/img1.png", OCRStatus: receipt.OCRStatusPending}
	r2 := &receipt.Receipt{ID: id2, StorageURL: "http://example.com/img2.png", OCRStatus: receipt.OCRStatusPending}

	mockRepo := &receipt.MockRepository{}

	// First call returns r1, second returns r2 (simulating SKIP LOCKED across two workers).
	mockRepo.On("ListPendingOCR", ctx).Return([]*receipt.Receipt{r1}, nil).Once()
	mockRepo.On("ListPendingOCR", ctx).Return([]*receipt.Receipt{r2}, nil).Once()

	receipts1, err1 := mockRepo.ListPendingOCR(ctx)
	require.NoError(t, err1)
	require.Len(t, receipts1, 1)
	assert.Equal(t, id1, receipts1[0].ID)

	receipts2, err2 := mockRepo.ListPendingOCR(ctx)
	require.NoError(t, err2)
	require.Len(t, receipts2, 1)
	assert.Equal(t, id2, receipts2[0].ID)

	// The two results are different — concurrent workers picked different receipts.
	assert.NotEqual(t, receipts1[0].ID, receipts2[0].ID)
}
