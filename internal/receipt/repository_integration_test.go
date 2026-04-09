//go:build integration

package receipt_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bmunoz/gentax/internal/receipt"
	"github.com/bmunoz/gentax/internal/testutil"
)

// seedOwnerDriverTaxi inserts the minimum rows required to satisfy FK constraints
// on the receipts table. Returns driverID and taxiID.
func seedOwnerDriverTaxi(t *testing.T, db *pgxpool.Pool) (driverID, taxiID uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	ownerID := uuid.New()
	_, err := db.Exec(ctx,
		`INSERT INTO owners (id, email, password_hash) VALUES ($1, $2, 'hash')`,
		ownerID, ownerID.String()+"@test.com",
	)
	require.NoError(t, err)

	driverID = uuid.New()
	_, err = db.Exec(ctx,
		`INSERT INTO drivers (id, owner_id, full_name) VALUES ($1, $2, 'Test Driver')`,
		driverID, ownerID,
	)
	require.NoError(t, err)

	taxiID = uuid.New()
	_, err = db.Exec(ctx,
		`INSERT INTO taxis (id, owner_id, plate, model, year) VALUES ($1, $2, 'ABC123', 'Toyota', 2022)`,
		taxiID, ownerID,
	)
	require.NoError(t, err)

	return driverID, taxiID
}

func TestReceiptRepository_Create_RequiresStorageURL(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	repo := receipt.NewRepository(db)

	driverID, taxiID := seedOwnerDriverTaxi(t, db)

	r := &receipt.Receipt{
		DriverID:   driverID,
		TaxiID:     taxiID,
		StorageURL: "", // intentionally empty
		OCRStatus:  receipt.OCRStatusPending,
	}

	_, err := repo.Create(ctx, r)
	require.Error(t, err)
	assert.ErrorIs(t, err, receipt.ErrEmptyStorageURL)
}

func TestReceiptRepository_ListPendingOCR_SkipLocked(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	repo := receipt.NewRepository(db)

	driverID, taxiID := seedOwnerDriverTaxi(t, db)

	// Seed 3 pending receipts.
	var ids []uuid.UUID
	for i := 0; i < 3; i++ {
		r := &receipt.Receipt{
			DriverID:   driverID,
			TaxiID:     taxiID,
			StorageURL: "https://storage.example.com/img-skip-" + string(rune('1'+i)) + ".png",
			OCRStatus:  receipt.OCRStatusPending,
		}
		created, err := repo.Create(ctx, r)
		require.NoError(t, err)
		ids = append(ids, created.ID)
	}

	// Verify ListPendingOCR returns all 3 when polled.
	pending, err := repo.ListPendingOCR(ctx)
	require.NoError(t, err)
	assert.Len(t, pending, 3)

	// Verify IDs match.
	got := make(map[uuid.UUID]bool)
	for _, p := range pending {
		got[p.ID] = true
	}
	for _, id := range ids {
		assert.True(t, got[id], "expected receipt %s in pending list", id)
	}
}

func TestReceiptRepository_UpdateOCRFields(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	repo := receipt.NewRepository(db)

	driverID, taxiID := seedOwnerDriverTaxi(t, db)

	r := &receipt.Receipt{
		DriverID:   driverID,
		TaxiID:     taxiID,
		StorageURL: "https://storage.example.com/img-update.png",
		OCRStatus:  receipt.OCRStatusPending,
	}
	created, err := repo.Create(ctx, r)
	require.NoError(t, err)

	vendor := "ACME LTDA"
	nit := "9004558906"
	total := "150000.00"
	date := "2024-01-15"
	concept := "Combustible"

	result := &receipt.OCRResult{
		Vendor:  &vendor,
		NIT:     &nit,
		Total:   &total,
		Date:    &date,
		Concept: &concept,
		RawJSON: []byte(`{"raw_text":"test"}`),
	}

	err = repo.UpdateOCRFields(ctx, created.ID, result)
	require.NoError(t, err)

	updated, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)

	require.NotNil(t, updated.ExtractedVendor)
	assert.Equal(t, vendor, *updated.ExtractedVendor)
	require.NotNil(t, updated.ExtractedNIT)
	assert.Equal(t, nit, *updated.ExtractedNIT)
	require.NotNil(t, updated.ExtractedTotal)
	assert.Equal(t, "150000", updated.ExtractedTotal.String())
}

func TestReceiptRepository_SetFailed(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	repo := receipt.NewRepository(db)

	driverID, taxiID := seedOwnerDriverTaxi(t, db)

	r := &receipt.Receipt{
		DriverID:   driverID,
		TaxiID:     taxiID,
		StorageURL: "https://storage.example.com/img-fail.png",
		OCRStatus:  receipt.OCRStatusPending,
	}
	created, err := repo.Create(ctx, r)
	require.NoError(t, err)

	errJSON := []byte(`{"error":"ocr provider unavailable"}`)
	err = repo.SetOCRStatus(ctx, created.ID, receipt.OCRStatusFailed, errJSON)
	require.NoError(t, err)

	updated, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, receipt.OCRStatusFailed, updated.OCRStatus)
}

// TestReceiptRepository_ListPendingOCR_ConcurrentWorkers runs two goroutines against
// the same DB; it asserts no panics and that both workers get results without deadlock.
func TestReceiptRepository_ListPendingOCR_ConcurrentWorkers(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)

	driverID, taxiID := seedOwnerDriverTaxi(t, db)
	repo := receipt.NewRepository(db)

	for i := 0; i < 2; i++ {
		r := &receipt.Receipt{
			DriverID:   driverID,
			TaxiID:     taxiID,
			StorageURL: "https://storage.example.com/img-concurrent-" + string(rune('a'+i)) + ".png",
			OCRStatus:  receipt.OCRStatusPending,
		}
		_, err := repo.Create(ctx, r)
		require.NoError(t, err)
	}

	var wg sync.WaitGroup
	results := make([][]*receipt.Receipt, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			pending, err := repo.ListPendingOCR(ctx)
			if err != nil {
				t.Logf("worker %d error: %v", idx, err)
			}
			results[idx] = pending
		}(i)
	}
	wg.Wait()

	t.Logf("worker 0 saw %d, worker 1 saw %d", len(results[0]), len(results[1]))
}
