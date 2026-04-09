//go:build integration

package testutil_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/bmunoz/gentax/internal/expense"
	"github.com/bmunoz/gentax/internal/receipt"
	"github.com/bmunoz/gentax/internal/testutil"
)

// TestE2E_ExpenseFullFlow exercises the complete expense lifecycle:
//
//  1. Seed owner, taxi, driver, expense category
//  2. Create a receipt via the receipt repository
//  3. Create an expense via expense.Service.Create
//  4. Process the receipt (mock OCR returns valid DIAN fields)
//  5. Assert receipt ocr_status=done, extracted_total set
//  6. Confirm expense (driver) → status=confirmed
//  7. Assert status=confirmed
//  8. Approve expense (owner) → status=approved
//  9. Assert status=approved, reviewed_by set
func TestE2E_ExpenseFullFlow(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	// ── Seed ────────────────────────────────────────────────────────────────────
	owner := testutil.CreateOwner(t, pool)
	taxi := testutil.CreateTaxi(t, pool, owner.ID)
	driver := testutil.CreateDriver(t, pool, owner.ID)
	category := testutil.CreateExpenseCategory(t, pool, owner.ID)

	// ── Repositories & services ──────────────────────────────────────────────────
	receiptRepo := receipt.NewRepository(pool)
	expenseRepo := expense.NewRepository(pool)
	expenseSvc := expense.NewService(expenseRepo)

	// ── Mock dependencies ────────────────────────────────────────────────────────
	mockStorage := &receipt.MockStorageClient{}
	mockOCR := &receipt.MockOCRClient{}

	// Storage mock: Download returns dummy image bytes for any URL.
	mockStorage.On("Download", mock.Anything, mock.AnythingOfType("string")).
		Return([]byte("fake-image-bytes"), nil)

	// OCR mock: returns valid DIAN fields.
	total := "85000"
	date := "01/04/2026"
	vendor := "Estación de Servicio ABC"
	nit := "900123456-7"
	cufe := "aabbcc" // short for test, not validated by repo
	concept := "Combustible"
	ocrResult := &receipt.OCRResult{
		Total:   &total,
		Date:    &date,
		Vendor:  &vendor,
		NIT:     &nit,
		CUFE:    &cufe,
		Concept: &concept,
		RawJSON: mustMarshal(map[string]string{
			"total":   total,
			"date":    date,
			"vendor":  vendor,
			"nit":     nit,
			"cufe":    cufe,
			"concept": concept,
		}),
	}
	mockOCR.On("ExtractData", mock.Anything, []byte("fake-image-bytes")).
		Return(ocrResult, nil)

	// Build processor with mocks.
	processor := receipt.NewProcessor(receiptRepo, mockOCR, mockStorage, nil)

	// ── Step 1: create receipt ────────────────────────────────────────────────────
	storageURL := "file:///data/receipts/" + uuid.New().String() + ".jpg"
	rec, err := receiptRepo.Create(ctx, &receipt.Receipt{
		DriverID:   driver.ID,
		TaxiID:     taxi.ID,
		StorageURL: storageURL,
		OCRStatus:  receipt.OCRStatusPending,
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, rec.ID)

	// ── Step 2: create expense ────────────────────────────────────────────────────
	exp, err := expenseSvc.Create(ctx, expense.CreateInput{
		OwnerID:    owner.ID,
		DriverID:   driver.ID,
		TaxiID:     taxi.ID,
		CategoryID: category.ID,
		ReceiptID:  rec.ID,
		Notes:      "E2E test expense",
	})
	require.NoError(t, err)
	require.Equal(t, expense.StatusPending, exp.Status)

	// ── Step 3: process OCR ───────────────────────────────────────────────────────
	err = processor.Process(ctx, rec.ID)
	require.NoError(t, err)

	// Verify receipt fields updated.
	updated, err := receiptRepo.GetByID(ctx, rec.ID)
	require.NoError(t, err)
	assert.Equal(t, receipt.OCRStatusDone, updated.OCRStatus, "ocr_status should be done")
	require.NotNil(t, updated.ExtractedTotal, "extracted_total should be set")
	assert.Equal(t, "85000", updated.ExtractedTotal.String())

	// ── Step 4: confirm expense (driver side) ────────────────────────────────────
	err = expenseSvc.Confirm(ctx, exp.ID, driver.ID)
	require.NoError(t, err)

	confirmed, err := expenseSvc.GetByID(ctx, exp.ID, owner.ID)
	require.NoError(t, err)
	assert.Equal(t, expense.StatusConfirmed, confirmed.Status, "expense should be confirmed")

	// ── Step 5: approve expense (owner side) ─────────────────────────────────────
	err = expenseSvc.Approve(ctx, exp.ID, owner.ID)
	require.NoError(t, err)

	approved, err := expenseSvc.GetByID(ctx, exp.ID, owner.ID)
	require.NoError(t, err)
	assert.Equal(t, expense.StatusApproved, approved.Status, "expense should be approved")
	require.NotNil(t, approved.ReviewedBy, "reviewed_by should be set")
	assert.Equal(t, owner.ID, *approved.ReviewedBy)

	// Verify mocks were called.
	mockOCR.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func mustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
